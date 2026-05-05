// Package bridge holds the gRPC client that talks to the on-robot
// ROS 2 bridge node. The contract is defined in proto/agent_bridge.proto;
// generated code lives in pb/.
//
// Sprint status (S1): scaffolding only. Today the telemetry pump
// uses a stub ingest loop. Wiring this client into pump.runIngest is
// the first deliverable of S1.
package bridge
