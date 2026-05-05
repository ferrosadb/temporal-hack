"""Synthetic /battery_state publisher for sim runs.

TurtleBot3 + Gazebo does not by default publish a sensor_msgs/BatteryState
topic. The bridge needs *something* to subscribe to so the end-to-end
data path is real. This node emits a slow battery drain on /battery_state
and is intended ONLY for sim use. Production robots emit their own
battery telemetry from real hardware.
"""

from __future__ import annotations

import time

import rclpy
from rclpy.node import Node
from sensor_msgs.msg import BatteryState


class SimBattery(Node):
    def __init__(self) -> None:
        super().__init__("sim_battery")
        self.pub = self.create_publisher(BatteryState, "/battery_state", 10)
        self.timer = self.create_timer(1.0, self._tick)
        self.charge_pct = 100.0
        self.start = time.monotonic()

    def _tick(self) -> None:
        elapsed = time.monotonic() - self.start
        # Drain 1% per minute. Wraps at 0 back to 100 to keep sim long-running.
        self.charge_pct = max(0.0, 100.0 - (elapsed / 60.0))
        if self.charge_pct <= 0.0:
            self.charge_pct = 100.0
            self.start = time.monotonic()
        msg = BatteryState()
        msg.voltage = 12.0 - (1.0 - self.charge_pct / 100.0) * 2.5
        msg.percentage = self.charge_pct / 100.0
        msg.power_supply_status = BatteryState.POWER_SUPPLY_STATUS_DISCHARGING
        msg.present = True
        self.pub.publish(msg)


def main() -> None:  # pragma: no cover
    rclpy.init()
    n = SimBattery()
    try:
        rclpy.spin(n)
    finally:
        n.destroy_node()
        rclpy.shutdown()


if __name__ == "__main__":
    main()
