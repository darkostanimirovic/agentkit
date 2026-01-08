# Contributing to AgentKit

Thank you for your interest in contributing to AgentKit! This document provides guidelines and instructions for contributing.

## Getting Started

1. Fork the repository
2. Clone your fork: `git clone https://github.com/YOUR_USERNAME/agentkit.git`
3. Create a new branch: `git checkout -b feature/your-feature-name`
4. Make your changes
5. Run tests: `go test ./...`
6. Commit your changes: `git commit -am 'Add some feature'`
7. Push to the branch: `git push origin feature/your-feature-name`
8. Submit a pull request

## Development Guidelines

### Code Style

- Follow standard Go conventions and idioms
- Run `go fmt` before committing
- Use meaningful variable and function names
- Add comments for exported functions and types

### Testing

- Write tests for new functionality
- Ensure all tests pass before submitting PR
- Maintain or improve code coverage
- Use `MockLLM` for testing agent behavior

### Pull Request Process

1. Update the README.md with details of changes if needed
2. Update tests to reflect your changes
3. The PR will be merged once reviewed and approved

## Code of Conduct

- Be respectful and inclusive
- Welcome newcomers and help them get started
- Focus on constructive feedback
- Assume good intentions

## Questions?

Feel free to open an issue for any questions or concerns.
