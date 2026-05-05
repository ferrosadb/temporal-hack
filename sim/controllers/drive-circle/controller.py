"""drive-circle: publish a constant Twist that drives a TurtleBot3 in a circle.

This node is the simplest possible OTA payload: it claims /cmd_vel and emits
a fixed forward + angular velocity. Visible motion in Gazebo, no sensors.

Linear  0.15 m/s   ~= TurtleBot3 burger comfortable speed
Angular 0.5 rad/s  ~= circle of radius 0.3 m, period ~12.5 s
"""

import rclpy
from rclpy.node import Node
from geometry_msgs.msg import Twist


class DriveCircle(Node):
    def __init__(self):
        super().__init__("drive_circle")
        self.pub = self.create_publisher(Twist, "/cmd_vel", 10)
        self.timer = self.create_timer(0.1, self.tick)
        self.get_logger().info("drive-circle controller up")

    def tick(self):
        msg = Twist()
        msg.linear.x = 0.15
        msg.angular.z = 0.5
        self.pub.publish(msg)


def main():
    rclpy.init()
    node = DriveCircle()
    try:
        rclpy.spin(node)
    finally:
        node.destroy_node()
        rclpy.shutdown()


if __name__ == "__main__":
    main()
