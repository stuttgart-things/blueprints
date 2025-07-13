// Kubernetes Microservice module for building and staging container images
//
// This module provides a high-level abstraction for working with container images
// tailored for Kubernetes microservices, using Dagger as the execution engine.
//
// It offers two primary functions:
//   - BakeImage: Builds and optionally pushes a Docker image from source code
//                with support for extra build directories and custom Dockerfile paths.
//   - StageImage: Stages (copies) an existing image between registries, optionally
//                 using Docker config authentication or username/password pairs.
//                 Supports insecure registries and custom platforms.
//
// Typical usage scenarios include:
//   - Building a microservice image in CI/CD pipelines and pushing directly to a registry
//   - Promoting (staging) images between registries (e.g., dev -> staging -> prod)
//   - Supporting custom build contexts through additional directories
//
// Internally, this module delegates to the 'Docker' module for building/pushing images,
// and the 'Crane' module for staging images between registries.
//
// Example workflows:
//   - Bake a microservice image and push to a dev registry
//   - Stage a built image to a production registry using secure or insecure connections
//
// This module is designed for integration in CI pipelines, platform automation, or
// developer tooling around Kubernetes microservices.

package main

type KubernetesMicroservice struct{}
