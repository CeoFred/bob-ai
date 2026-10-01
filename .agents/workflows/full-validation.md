# Workflow: Full Project Validation

Comprehensive validation pipeline to run before committing changes.

## Steps

1. **Frontend Typecheck & Build**:
   ```bash
   cd web && npm run build
   ```

2. **Backend Tests**:
   ```bash
   go test -v ./...
   ```

3. **Backend Binary Build**:
   ```bash
   mkdir -p bin
   go build -o bin/bob cmd/bob/main.go
   ```

4. **Security Policy Check**:
   ```bash
   go test -v ./internal/security/...
   ```

5. **Update Knowledge Graph**:
   ```bash
   $(cat graphify-out/.graphify_python) -m graphify --update
   ```
