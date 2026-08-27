# Development Tools

This directory contains development tools, code generation scripts, and mock servers.

## Structure

```
tools/
├── codegen/    # Code generation scripts
└── mocks/      # Mock servers and test data generators
```

## Code Generation

Tools for generating boilerplate code, models, and API clients.

### Examples
- Generate Riverpod providers from models
- Generate API client code from OpenAPI specs
- Generate database migration scripts
- Generate test fixtures

### Usage
```bash
cd tools/codegen
# TODO: Add codegen commands when implemented
```

## Mock Servers

Mock servers for local development and testing.

### Use Cases
- Mock payment gateway (Midtrans)
- Mock external APIs
- Generate realistic test data
- Simulate network conditions

### Usage
```bash
cd tools/mocks
# TODO: Add mock server commands when implemented
```

## Best Practices

1. **Document scripts**: Add clear README for each tool
2. **Make scripts reusable**: Use configuration files
3. **Version control**: Keep tools in git
4. **Add examples**: Provide usage examples
5. **Keep it simple**: Don't over-engineer tools

## Future Additions

- [ ] OpenAPI code generator
- [ ] Database seeder script
- [ ] Mock payment gateway server
- [ ] Test data generator
- [ ] Performance testing tools
