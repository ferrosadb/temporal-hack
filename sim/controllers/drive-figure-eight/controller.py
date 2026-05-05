"""drive-figure-eight: drive a TurtleBot3 in a continuous figure-eight.

Same linear/angular magnitudes as drive-circle, but flips the sign of
angular.z every `half_period_s` seconds. The robot draws one full circle
in either direction before reversing, producing an approximate figure-8.

half_period_s = 12.5 ≈ one full revolution at angular.z = 0.5 rad/s,
so each lobe of the eight is a complete loop. Tune the value to make
the lobes overlap more or less.
"""

import rclpy
from rclpy.node import Node
from geometry_msgs.msg import Twist


class DriveFigureEight(Node):
    def __init__(self):
        super().__init__("drive_figure_eight")
        self.pub = self.create_publisher(Twist, "/cmd_vel", 10)
        self.start = self.get_clock().now()
        self.half_period_s = 12.5
        self.timer = self.create_timer(0.1, self.tick)
        self.get_logger().info("drive-figure-eight controller up")

    def tick(self):
        elapsed = (self.get_clock().now() - self.start).nanoseconds / 1e9
        sign = 1.0 if int(elapsed // self.half_period_s) % 2 == 0 else -1.0
        msg = Twist()
        msg.linear.x = 0.15
        msg.angular.z = 0.5 * sign
        self.pub.publish(msg)


def main():
    rclpy.init()
    node = DriveFigureEight()
    try:
        rclpy.spin(node)
    finally:
        node.destroy_node()
        rclpy.shutdown()


if __name__ == "__main__":
    main()
