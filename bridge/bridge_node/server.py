"""ROS 2 → gRPC bridge.

Subscribes to a configured set of ROS 2 topics and exposes them to the
Go robot agent over a gRPC server bound to a Unix domain socket.

Sprint status (S1): minimal viable subscriber for one topic
(/battery_state from sensor_msgs). Stream→topic mapping is hard-coded
here; configuration support lands in S2.

Run inside the ROS 2 environment:

    source /opt/ros/humble/setup.bash
    python -m bridge_node.server --socket /run/temporal-hack-bridge.sock
"""

from __future__ import annotations

import argparse
import logging
import os
import queue
import signal
import threading
import time
from concurrent import futures

import grpc

from .proto import agent_bridge_pb2 as pb
from .proto import agent_bridge_pb2_grpc as pbg

try:
    import rclpy
    from rclpy.node import Node
    from sensor_msgs.msg import BatteryState
    HAVE_ROS = True
except ImportError:  # pragma: no cover
    HAVE_ROS = False


log = logging.getLogger("bridge")


STREAM_TO_TOPIC = {
    "battery": ("/battery_state", "ros2:sensor_msgs/BatteryState@v1"),
    # TODO(S2): "pose" -> ("/odom", "ros2:nav_msgs/Odometry@v1"),
    # TODO(S2): "diag" -> ("/diagnostics", "ros2:diagnostic_msgs/DiagnosticArray@v1"),
}


class TopicFanout:
    """Thread-safe fanout: ROS callbacks push events; gRPC subscribers pull."""

    def __init__(self) -> None:
        self._subs: dict[int, queue.Queue] = {}
        self._next = 0
        self._lock = threading.Lock()

    def add(self) -> tuple[int, queue.Queue]:
        q: queue.Queue = queue.Queue(maxsize=1024)
        with self._lock:
            self._next += 1
            sid = self._next
            self._subs[sid] = q
        return sid, q

    def remove(self, sid: int) -> None:
        with self._lock:
            self._subs.pop(sid, None)

    def push(self, event) -> None:  # event is a pb.TopicEvent
        with self._lock:
            subs = list(self._subs.values())
        for q in subs:
            try:
                q.put_nowait(event)
            except queue.Full:
                # Drop on backpressure; agent has its own buffer.
                pass


class BridgeService:
    """gRPC service implementation. Implements pbg.BridgeServicer at runtime."""

    def __init__(self, fanout: TopicFanout) -> None:
        self.fanout = fanout

    def Subscribe(self, request, context):
        sid, q = self.fanout.add()
        try:
            while context.is_active():
                try:
                    ev = q.get(timeout=1.0)
                except queue.Empty:
                    continue
                yield ev
        finally:
            self.fanout.remove(sid)

    def Health(self, request, context):
        return pb.HealthResponse(
            state=pb.HealthResponse.HEALTHY,
            detail="ok",
            active_streams=list(STREAM_TO_TOPIC.keys()),
        )


class BatterySubscriber(Node):  # pragma: no cover -- requires ROS at runtime
    def __init__(self, fanout: TopicFanout) -> None:
        super().__init__("temporal_hack_bridge")
        self.fanout = fanout
        self.create_subscription(BatteryState, "/battery_state", self._on_msg, 10)

    def _on_msg(self, msg: BatteryState) -> None:
        import json as _json
        from google.protobuf.timestamp_pb2 import Timestamp
        now = Timestamp()
        now.GetCurrentTime()
        # JSON payload — small, debuggable in mosquitto_sub. Schema string
        # tells the cloud-side decoder which keys to expect.
        body = _json.dumps({
            "voltage": float(msg.voltage),
            "percentage": float(msg.percentage),
            "present": bool(msg.present),
            "status": int(msg.power_supply_status),
        }).encode("utf-8")
        ev = pb.TopicEvent(
            stream="battery",
            captured_at=now,
            payload=body,
            payload_schema="json:battery@v1",
        )
        self.fanout.push(ev)


def serve(addr: str) -> None:
    """Run the bridge gRPC server.

    `addr` accepts either a bare ``host:port`` for TCP, or a path /
    ``unix://<path>`` form for a Unix domain socket. Determined by
    whether the value contains a colon and no slash.
    """
    fanout = TopicFanout()

    server = grpc.server(futures.ThreadPoolExecutor(max_workers=8))
    pbg.add_BridgeServicer_to_server(BridgeService(fanout), server)

    if addr.startswith("unix://"):
        listen = addr
        sock_path = addr[len("unix://"):]
        if os.path.exists(sock_path):
            os.unlink(sock_path)
    elif ":" in addr and "/" not in addr:
        listen = addr  # bare host:port -> TCP
    else:
        # bare path -> Unix domain socket
        if os.path.exists(addr):
            os.unlink(addr)
        listen = f"unix://{addr}"
    server.add_insecure_port(listen)
    server.start()
    log.info("bridge listening on %s", listen)

    if HAVE_ROS:
        rclpy.init(args=None)
        sub = BatterySubscriber(fanout)
        ros_thread = threading.Thread(target=lambda: rclpy.spin(sub), daemon=True)
        ros_thread.start()
    else:  # pragma: no cover
        log.warning("rclpy not available; bridge will not subscribe to ROS topics")

    stop = threading.Event()
    for sig in (signal.SIGINT, signal.SIGTERM):
        signal.signal(sig, lambda *_: stop.set())
    while not stop.is_set():
        time.sleep(0.5)
    server.stop(2.0)


def main() -> None:  # pragma: no cover
    p = argparse.ArgumentParser()
    # --listen is the modern arg (TCP host:port or unix:// path).
    # --socket kept for backwards compat with older callers.
    p.add_argument("--listen", default=None,
                   help="gRPC listen address: 'host:port' or 'unix:///path'")
    p.add_argument("--socket", default=None,
                   help="(deprecated) Unix domain socket path. Use --listen.")
    args = p.parse_args()
    addr = args.listen or args.socket or "/run/temporal-hack-bridge.sock"
    logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(name)s %(message)s")
    serve(addr)


if __name__ == "__main__":
    main()
