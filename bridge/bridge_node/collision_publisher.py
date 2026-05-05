"""collision_publisher: ROS /contacts -> MQTT events/{robot_id}/collision.

Subscribes to the ros_gz_interfaces/Contacts topic that gazebo's
ros_gz_bridge republishes from /perseverance/contacts. When a contact
is observed (any contact, not just rocks), publishes a small JSON
event to the cloud broker so the Temporal CollisionResponse workflow
can be triggered.

Debounces aggressive contact streams: the contact sensor fires every
simulation step while a collision is sustained, which would flood
MQTT and trigger many workflows. We collapse repeats within a 2s
window into a single event.
"""

from __future__ import annotations

import json
import os
import time

import paho.mqtt.client as mqtt
import rclpy
from rclpy.node import Node
from ros_gz_interfaces.msg import Contacts


DEBOUNCE_SEC = 2.0


class CollisionPublisher(Node):
    def __init__(self) -> None:
        super().__init__("collision_publisher")
        self.robot_id = os.environ.get("ROBOT_ID", "sim-robot-01")
        self.broker_host = os.environ.get("MQTT_HOST", "mqtt")
        self.broker_port = int(os.environ.get("MQTT_PORT", "1883"))
        self.topic = f"events/{self.robot_id}/collision"

        # paho-mqtt 1.x API (Ubuntu jammy ships 1.6); v2's
        # CallbackAPIVersion isn't available here.
        self.client = mqtt.Client(client_id=f"collision-pub-{self.robot_id}")
        self.client.connect(self.broker_host, self.broker_port, keepalive=30)
        self.client.loop_start()

        self.create_subscription(Contacts, "/contacts", self._on_contacts, 10)
        self._last_emit = 0.0
        self._counter = 0
        self.get_logger().info(
            f"collision_publisher up: ROS /contacts -> MQTT {self.broker_host}:{self.broker_port}/{self.topic}"
        )

    def _on_contacts(self, msg: Contacts) -> None:
        # Contacts message has a `contacts` field — a list. Empty list
        # means "no contacts this step"; only act on non-empty.
        if not msg.contacts:
            return
        now = time.monotonic()
        if now - self._last_emit < DEBOUNCE_SEC:
            return
        self._last_emit = now
        self._counter += 1
        partner = msg.contacts[0].collision2.name if msg.contacts else "unknown"
        body = json.dumps({
            "robot_id": self.robot_id,
            "at": time.time(),
            "count": self._counter,
            "partner": partner,
        })
        self.client.publish(self.topic, body, qos=1)
        self.get_logger().warn(f"collision (count={self._counter}, partner={partner})")


def main() -> None:  # pragma: no cover
    rclpy.init()
    node = CollisionPublisher()
    try:
        rclpy.spin(node)
    finally:
        node.client.loop_stop()
        node.client.disconnect()
        node.destroy_node()
        rclpy.shutdown()


if __name__ == "__main__":
    main()
