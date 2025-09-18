I've analyzed the Maestro codebase and created a comprehensive CLAUDE.md file. The codebase is a sophisticated multi-cluster Kubernetes orchestration system with the following key characteristics:

**Project**: Maestro - A scalable Kubernetes resource orchestration system using CloudEvents
**Language**: Go 1.24.4
**Architecture**: Event-driven with dual broker support (MQTT/gRPC) and consistent hashing

**Key Commands Found**:
- `make binary` - Build
- `make test` / `make test-integration` / `make e2e-test` - Testing  
- `make lint` - Linting
- `make verify` - Source verification
- `./maestro migration` - Database migrations

The system is designed for enterprise scale (200k+ clusters) using PostgreSQL instead of etcd, with sophisticated load balancing and multi-broker message transport capabilities.
