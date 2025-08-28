# Contributing to Ollama LanCache

Thank you for your interest in contributing to Ollama LanCache! This document provides guidelines and information for contributors.

## 🚀 Quick Start

1. **Fork the repository** on GitHub
2. **Clone your fork** locally:
   ```bash
   git clone https://github.com/YOUR_USERNAME/ollama-lancache.git
   cd ollama-lancache
   ```
3. **Create a feature branch**:
   ```bash
   git checkout -b feature/your-feature-name
   ```
4. **Make your changes** and commit them
5. **Push to your fork** and submit a pull request

## 🛠️ Development Setup

### Prerequisites

- Go 1.21 or higher
- Git
- Make (optional, but recommended)

### Environment Setup

```bash
# Clone the repository
git clone https://github.com/jjasghar/ollama-lancache.git
cd ollama-lancache

# Install dependencies
go mod download

# Build the project
make build
# OR
go build -o ollama-lancache .

# Run tests
make test
# OR
go test ./...

# Run the application
./ollama-lancache serve --port 8080
```

### Development Commands

```bash
# Format code
make fmt
go fmt ./...

# Run linter
make lint
golangci-lint run

# Run tests with coverage
make test-coverage
go test -coverprofile=coverage.out ./...

# Build for multiple platforms
make build-all

# Clean build artifacts
make clean
```

## 📝 Code Style and Standards

### Go Code Guidelines

- Follow standard Go formatting (`gofmt`)
- Use meaningful variable and function names
- Add comments for exported functions and complex logic
- Keep functions small and focused
- Handle errors appropriately
- Use Go modules for dependency management

### Example Code Style

```go
// Good: Clear function name and documentation
// ServeModel handles HTTP requests for model downloads
func (s *ModelServer) ServeModel(w http.ResponseWriter, r *http.Request) {
    modelName := r.URL.Query().Get("model")
    if modelName == "" {
        http.Error(w, "model parameter required", http.StatusBadRequest)
        return
    }
    
    // Handle the request...
}

// Good: Proper error handling
func (s *ModelServer) loadModels() error {
    files, err := os.ReadDir(s.modelsDir)
    if err != nil {
        return fmt.Errorf("failed to read models directory: %w", err)
    }
    
    // Process files...
    return nil
}
```

### Commit Message Guidelines

We follow the [Conventional Commits](https://www.conventionalcommits.org/) specification:

```
type(scope): description

[optional body]

[optional footer(s)]
```

**Types:**
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `style`: Code style changes (formatting, etc.)
- `refactor`: Code refactoring
- `test`: Adding or updating tests
- `chore`: Maintenance tasks

**Examples:**
```
feat(server): add model distribution endpoint
fix(client): handle Windows path separators correctly
docs(readme): update installation instructions
test(cache): add unit tests for cache management
```

## 🧪 Testing

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with verbose output
go test -v ./...

# Run tests with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run specific test package
go test ./internal/cache/

# Run specific test function
go test -run TestCacheStats ./internal/cache/
```

### Writing Tests

- Write tests for all new functionality
- Use table-driven tests where appropriate
- Include both positive and negative test cases
- Mock external dependencies
- Aim for high test coverage (>80%)

**Test Example:**
```go
func TestModelServer_ListModels(t *testing.T) {
    tests := []struct {
        name     string
        setup    func() *ModelServer
        want     int
        wantErr  bool
    }{
        {
            name: "valid models directory",
            setup: func() *ModelServer {
                return &ModelServer{modelsDir: "testdata/models"}
            },
            want:    3,
            wantErr: false,
        },
        {
            name: "empty directory",
            setup: func() *ModelServer {
                return &ModelServer{modelsDir: "testdata/empty"}
            },
            want:    0,
            wantErr: false,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            s := tt.setup()
            got, err := s.ListModels()
            
            if (err != nil) != tt.wantErr {
                t.Errorf("ListModels() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            
            if len(got) != tt.want {
                t.Errorf("ListModels() got %d models, want %d", len(got), tt.want)
            }
        })
    }
}
```

## 📚 Documentation

### Code Documentation

- Document all exported functions, types, and variables
- Use clear, concise language
- Provide examples for complex functions
- Keep documentation up-to-date with code changes

### Documentation Updates

When adding new features or changing existing ones:

1. Update the README.md if user-facing changes
2. Update inline code documentation
3. Add or update examples
4. Update CLI help text if applicable

## 🐛 Bug Reports

When reporting bugs, please include:

1. **Description**: Clear description of the issue
2. **Steps to Reproduce**: Detailed steps to reproduce the bug
3. **Expected Behavior**: What you expected to happen
4. **Actual Behavior**: What actually happened
5. **Environment**: OS, Go version, Ollama version
6. **Logs**: Relevant log output or error messages

**Bug Report Template:**
```
## Bug Description
A clear description of what the bug is.

## Steps to Reproduce
1. Go to '...'
2. Click on '....'
3. Scroll down to '....'
4. See error

## Expected Behavior
A clear description of what you expected to happen.

## Actual Behavior
A clear description of what actually happened.

## Environment
- OS: [e.g. Ubuntu 20.04, Windows 11, macOS 13]
- Go Version: [e.g. 1.21.5]
- Ollama Version: [e.g. 0.1.17]
- Ollama LanCache Version: [e.g. 1.0.0]

## Additional Context
Add any other context about the problem here.
```

## 💡 Feature Requests

For feature requests, please provide:

1. **Use Case**: Describe your use case and problem
2. **Proposed Solution**: Your proposed solution
3. **Alternatives**: Alternative solutions you've considered
4. **Additional Context**: Any additional context or screenshots

## 🔀 Pull Request Process

### Before Submitting

1. **Create an issue** for discussion (for significant changes)
2. **Fork the repository** and create a feature branch
3. **Write tests** for your changes
4. **Update documentation** as needed
5. **Run tests** and ensure they pass
6. **Run linting** and fix any issues

### Pull Request Guidelines

1. **Title**: Use a clear, descriptive title
2. **Description**: Provide a detailed description of changes
3. **Link Issues**: Reference related issues using `#issue-number`
4. **Small Changes**: Keep PRs focused and relatively small
5. **Test Coverage**: Ensure adequate test coverage

### Pull Request Template

```markdown
## Description
Brief description of the changes.

## Type of Change
- [ ] Bug fix (non-breaking change that fixes an issue)
- [ ] New feature (non-breaking change that adds functionality)
- [ ] Breaking change (fix or feature that would cause existing functionality to change)
- [ ] Documentation update

## Related Issues
Fixes #(issue number)

## Testing
- [ ] Unit tests added/updated
- [ ] Integration tests added/updated
- [ ] Manual testing completed

## Checklist
- [ ] Code follows the project's style guidelines
- [ ] Self-review of code completed
- [ ] Code is commented, particularly hard-to-understand areas
- [ ] Documentation updated as needed
- [ ] Tests added/updated and all tests pass
- [ ] No new warnings or errors introduced
```

### Review Process

1. **Automated Checks**: CI/CD will run tests and linting
2. **Code Review**: Maintainers will review your code
3. **Feedback**: Address any requested changes
4. **Approval**: PR will be approved once ready
5. **Merge**: Maintainers will merge the PR

## 📧 Communication

- **GitHub Issues**: For bug reports and feature requests
- **GitHub Discussions**: For questions and general discussion
- **Pull Requests**: For code contributions

## 🎯 Areas for Contribution

We welcome contributions in these areas:

### High Priority
- **Performance improvements** for large model transfers
- **Better error handling** and user feedback
- **Cross-platform compatibility** fixes
- **Security enhancements**

### Medium Priority
- **Additional client scripts** (Python, Ruby, etc.)
- **Web UI improvements** for the model browser
- **Monitoring and metrics** collection
- **Configuration management** enhancements

### Low Priority
- **Additional storage backends** (S3, GCS, etc.)
- **Load balancing** capabilities
- **Advanced caching strategies**
- **Plugin system** for extensibility

## 🏅 Recognition

Contributors will be:
- Listed in the project's contributor list
- Mentioned in release notes for significant contributions
- Given credit in documentation where appropriate

## ❓ Questions?

If you have questions about contributing, please:
1. Check existing documentation
2. Search through GitHub issues
3. Start a GitHub discussion
4. Contact the maintainers

Thank you for contributing to Ollama LanCache! 🎉
