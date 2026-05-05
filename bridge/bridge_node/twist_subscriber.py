"""twist_subscriber: MQTT cmd/{robot_id}/twist -> ROS /cmd_vel.

The cloud-side CollisionResponse workflow publishes Twist commands on
this MQTT topic at ~10 Hz for each phase (back-up, turn, forward).
This node deserialises and republishes onto ROS /cmd_vel; the
gazebo container's ros_gz_bridge forwards to the diff-drive plugin.

Wire format (JSON):
    {"linear_x": -0.3, "angular_z": 0}
"""

from __future__ import annotations

import json
import os

import paho.mqtt.client as mqtt
import rclpy
from rclpy.node import Node
from geometry_msgs.msg import Twist


class TwistSubscriber(Node):
    def __init__(self) -> None:
        super().__init__("twist_subscriber")
        self.robot_id = os.environ.get("ROBOT_ID", "sim-robot-01")
        self.broker_host = os.environ.get("MQTT_HOST", "mqtt")
        self.broker_port = int(os.environ.get("MQTT_PORT", "1883"))
        self.topic = f"cmd/{self.robot_id}/twist"

        self.pub = self.create_publisher(Twist, "/cmd_vel", 10)

        # paho-mqtt 1.x API (Ubuntu jammy ships 1.6).
        self.client = mqtt.Client(client_id=f"twist-sub-{self.robot_id}")
        self.client.on_message = self._on_message
        self.client.connect(self.broker_host, self.broker_port, keepalive=30)
        self.client.subscribe(self.topic, qos=1)
        self.client.loop_start()
        self.get_logger().info(
            f"twist_subscriber up: MQTT {self.broker_host}:{self.broker_port}/{self.topic} -> ROS /cmd_vel"
        )

    def _on_message(self, _client, _userdata, msg) -> None:
        try:
            body = json.loads(msg.payload)
        except Exception as e:
            self.get_logger().warn(f"bad twist payload: {e}")
            return
        t = Twist()
        t.linear.x = float(body.get("linear_x", 0.0))
        t.linear.y = float(body.get("linear_y", 0.0))
        t.linear.z = float(body.get("linear_z", 0.0))
        t.angular.x = float(body.get("angular_x", 0.0))
        t.angular.y = float(body.get("angular_y", 0.0))
        t.angular.z = float(body.get("angular_z", 0.0))
        self.pub.publish(t)


def main() -> None:  # pragma: no cover
    rclpy.init()
    node = TwistSubscriber()
    try:
        rclpy.spin(node)
    finally:
        node.client.loop_stop()
        node.client.disconnect()
        node.destroy_node()
        rclpy.shutdown()


if __name__ == "__main__":
    main()
