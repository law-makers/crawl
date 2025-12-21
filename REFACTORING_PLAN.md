# Comprehensive Refactoring & Code Cleanup Plan

**Status**: Phase 1.2 Complete ✅  
**Created**: December 21, 2025  
**Last Updated**: December 21, 2025
**Priority**: High - Code Organization & Maintainability

---

## Phase 2: Dependency Injection & Decoupling

### 2.1 Eliminate Global Variables

**Current Global State**:
```go
// ❌ BAD: Global cache
var GlobalCache *MemoryCache

// ❌ BAD: Global rate limiter
var GlobalLimiter = NewDomainLimiter(5.0, 10)
```

**New Pattern** (using dependency injection):
```go
// ✅ GOOD: Pass dependencies as parameters
func (d *DynamicScraper) Fetch(ctx context.Context, opts RequestOptions, cache Cache, limiter RateLimiter) error {
    // Use cache and limiter from parameters
}
```

**Implementation Steps**:

1. **Update Scraper Interface**:
   ```go
   type Scraper interface {
       Fetch(ctx context.Context, opts RequestOptions) (*PageData, error)
   }
   
   // Modify implementations to accept dependencies in constructors
   type DynamicScraper struct {
       cache       Cache
       limiter     RateLimiter
       browserPool *BrowserPool
       client      *http.Client
   }
   
   func NewDynamicScraper(
       cache Cache,
       limiter RateLimiter,
       browserPool *BrowserPool,
       client *http.Client,
   ) *DynamicScraper {
       return &DynamicScraper{
           cache: cache,
           limiter: limiter,
           browserPool: browserPool,
           client: client,
       }
   }
   ```

2. **Remove Global References**:
   - Remove `cache.GlobalCache` usage
   - Remove `limiter.GlobalLimiter` usage
   - Update all calling code to use injected dependencies

3. **Create Interface Abstractions**:
   ```go
   // internal/cache/interfaces.go
   type Cache interface {
       Get(key string) (*models.PageData, bool)
       Set(key string, data *models.PageData, ttl time.Duration)
       Delete(key string)
       Close() error
   }
   
   // internal/ratelimit/interfaces.go
   type RateLimiter interface {
       Wait(ctx context.Context, domain string) error
       MarkSuccess(domain string)
       MarkFailure(domain string)
   }
   ```

**Files to Update**:
- `internal/cache/cache.go` - Remove global, add interface
- `internal/ratelimit/limiter.go` - Remove global, add interface
- `internal/engine/static.go` - Update to use injected cache/limiter
- `internal/engine/dynamic.go` - Update to use injected cache/limiter
- `internal/cli/get.go` - Receive dependencies from app context

---

## Phase 3: Code Organization & Modularity

### 3.1 Refactor Engine Package Structure

**Current Issue**: Mixed concerns (static, dynamic, batch, extraction)

**New Structure**:
```
internal/engine/
├── interfaces.go         # Scraper interface (moved from engine.go)
├── errors.go             # Engine-specific errors
├── static/
│   ├── scraper.go        # StaticScraper implementation
│   ├── extractor.go      # HTML extraction logic
│   └── pool.go           # Connection pooling
├── dynamic/
│   ├── scraper.go        # DynamicScraper implementation
│   ├── browser.go        # Browser pool management
│   ├── extractor.go      # JS execution and extraction
│   └── options.go        # Dynamic scraper options
├── hybrid/
│   ├── scraper.go        # HybridScraper implementation
│   ├── detector.go       # Static vs Dynamic detection
│   └── strategy.go       # Mode selection strategy
├── batch/
│   ├── scraper.go        # Batch processing
│   ├── concurrency.go    # Concurrency calculation
│   └── grouping.go       # Domain grouping
└── metadata/
    ├── extractor.go      # Metadata extraction
    └── utils.go          # Utility functions
```

**Implementation Steps**:

1. **Create clear sub-packages** with single responsibilities
2. **Move implementations** to appropriate packages
3. **Define package-level interfaces** for cross-package communication
4. **Remove circular dependencies** (if any)

**Impact**:
- ✅ Better code organization
- ✅ Single responsibility principle
- ✅ Easier to find and modify code
- ✅ Better for parallel development

---

### 3.2 Refactor CLI Package Structure

**Current Issue**: All commands in one package, mixed concerns

**New Structure**:
```
internal/cli/
├── root.go               # Root command setup
├── cmd.go               # Command registration
├── flags/
│   ├── common.go        # Common flags
│   ├── scrape.go        # Scrape command flags
│   └── auth.go          # Auth command flags
├── commands/
│   ├── get/
│   │   ├── get.go       # Get command
│   │   ├── output.go    # Output formatting
│   │   └── extractors.go # Data extraction logic
│   ├── media/
│   │   ├── media.go     # Media command
│   │   └── download.go  # Download handler
│   ├── batch/
│   │   └── batch.go     # Batch command
│   ├── login/
│   │   └── login.go     # Login command
│   └── sessions/
│       ├── sessions.go  # Sessions command
│       ├── import.go    # Import handler
│       └── export.go    # Export handler
└── ui/
    ├── formatter.go     # Output formatting
    ├── colors.go        # ANSI colors (move from root.go)
    └── progress.go      # Progress bars
```

**Impact**:
- ✅ Each command has its own package
- ✅ Easier to navigate large CLI
- ✅ Reduced file size
- ✅ Better separation of concerns

---

### 3.3 Extract Shared Utilities

**New Package**: `internal/utils/`

```
internal/utils/
├── url/
│   ├── parser.go        # URL validation & parsing
│   └── normalization.go # URL normalization
├── headers/
│   ├── parser.go        # Header parsing
│   └── builder.go       # Header building
├── output/
│   ├── json.go          # JSON output
│   ├── csv.go           # CSV output
│   ├── markdown.go      # Markdown output
│   └── html.go          # HTML output
├── retry/
│   ├── backoff.go       # Backoff strategies
│   └── predicates.go    # Retry predicates
└── encoding/
    ├── hash.go          # Hashing utilities
    └── normalize.go     # Text normalization
```

**Move From**:
- URL validation from `cli/get.go` → `utils/url/parser.go`
- Header parsing from `cli/get.go` → `utils/headers/parser.go`
- Output formatting from `cli/get.go` → `utils/output/`

**Impact**:
- ✅ Reusable utilities
- ✅ Reduce duplication
- ✅ Single source of truth for common operations
- ✅ Better testability

---

## Phase 4: Testing & Error Handling

### 4.1 Implement Comprehensive Error Handling

**Current Issue**: Inconsistent error patterns, poor error context

**New Pattern**:
```go
// internal/errors/errors.go
type ErrorCode string

const (
    ErrNotFound       ErrorCode = "NOT_FOUND"
    ErrTimeout        ErrorCode = "TIMEOUT"
    ErrValidation     ErrorCode = "VALIDATION"
    ErrBrowserCrash   ErrorCode = "BROWSER_CRASH"
    ErrNetworkError   ErrorCode = "NETWORK_ERROR"
)

type AppError struct {
    Code      ErrorCode
    Message   string
    Underlying error
    Retry     bool
    Details   map[string]interface{}
}

func (e *AppError) Error() string { /* ... */ }
func (e *AppError) Unwrap() error { /* ... */ }
func (e *AppError) Is(target error) bool { /* ... */ }
```

**Implementation Steps**:
1. Create `internal/errors/` package
2. Define error codes and error types
3. Update all packages to use new error types
4. Add error wrapping with context
5. Update tests to check for specific errors

---

### 4.2 Add Integration Tests

**New Test Structure**:
```
tests/
├── integration/
│   ├── static_scraper_test.go
│   ├── dynamic_scraper_test.go
│   ├── batch_processing_test.go
│   ├── caching_test.go
│   └── fixtures/
│       ├── test_server.go      # Mock server
│       └── sample_pages/       # HTML fixtures
├── e2e/
│   ├── cli_test.go
│   ├── auth_test.go
│   └── media_test.go
└── testdata/
    ├── config.yaml
    └── cookies.json
```

---

## Phase 5: Documentation & Standards

### 5.1 Package Documentation

**Files to Create**:
- `internal/engine/README.md` - Engine architecture and usage
- `internal/auth/README.md` - Authentication system
- `internal/cache/README.md` - Caching strategy
- `internal/cli/README.md` - CLI command structure

### 5.2 Code Standards Document

**Create**: `CONTRIBUTING.md`
- Code style guide
- Naming conventions
- Error handling patterns
- Testing requirements
- Documentation standards

---

## Phase 6: Cleanup & Refactoring (Detailed)

### 6.1 Remove Deprecated Code

**Items to Remove**:
- Duplicate constants in multiple files
- Unused variables and functions
- Dead code paths
- Old configuration patterns

### 6.2 Consolidate Configuration

**Current Scattered Configuration**:
```
❌ engine/constants.go       - Rate limiting, timeout defaults
❌ cache/cache.go            - Cache size defaults
❌ cli/root.go               - Log level defaults
❌ auth/login.go             - Timeout defaults
❌ engine/browser_pool.go    - Browser options
```

**New Single Location**:
```
✅ internal/config/defaults.go - All defaults in one place
✅ internal/config/config.go    - All configuration loading
```

### 6.3 Update Comments and Documentation

**Standards**:
- All exported types must have documentation
- All exported functions must have documentation
- Package-level documentation in each package
- Complex logic must have inline comments
- Update outdated comments

---

## Implementation Timeline

### Week 1: Phase 1 (Foundation)
- [ ] Create `internal/config/` package
- [ ] Create `internal/app/` package
- [ ] Create `internal/browser/` package
- [ ] Update `cmd/crawl/main.go`

### Week 2: Phase 2 (Dependency Injection)
- [ ] Eliminate `GlobalCache`
- [ ] Eliminate `GlobalLimiter`
- [ ] Create interface abstractions
- [ ] Update scraper implementations

### Week 3: Phase 3 (Organization)
- [ ] Reorganize engine package
- [ ] Reorganize CLI package
- [ ] Create utils packages
- [ ] Update imports across codebase

### Week 4: Phase 4 & 5 (Testing & Docs)
- [ ] Create error handling package
- [ ] Add integration tests
- [ ] Write package documentation
- [ ] Create CONTRIBUTING.md

### Week 5: Phase 6 (Cleanup)
- [ ] Remove deprecated code
- [ ] Final review and testing
- [ ] Update README.md
- [ ] Performance testing

---

## Breaking Changes & Migration

### For Users
- **No breaking changes** - CLI interface stays the same
- **Improved configuration** - New config file support optional

### For Developers
- **Updated imports** - Some internal imports will change
- **Dependency injection** - New testing patterns
- **Error types** - New error handling (backward compatible via `Unwrap()`)

---

## Expected Outcomes

### Code Metrics Improvement
| Metric | Before | After | Target |
|--------|--------|-------|--------|
| Global Variables | 2+ | 0 | ✅ |
| Package Cohesion | Low | High | ✅ |
| Circular Dependencies | Potential | None | ✅ |
| Test Coverage | ~60% | ~80% | ✅ |
| Code Duplication | Medium | Low | ✅ |
| Configuration Locations | 4+ | 1 | ✅ |

### Quality Improvements
- ✅ **Easier to maintain** - Clear structure and responsibilities
- ✅ **Easier to test** - Dependency injection throughout
- ✅ **Easier to extend** - Modular packages, clear interfaces
- ✅ **Cross-platform** - Chrome detection works on all OS
- ✅ **Better documentation** - Each package self-documenting
- ✅ **Single source of truth** - No duplicated configuration

---

## Quick Reference: File Changes Summary

### Files to Create (NEW)
- `internal/config/` (entire package)
- `internal/app/` (entire package)
- `internal/browser/` (entire package)
- `internal/errors/` (entire package)
- `internal/utils/` (entire package)
- `CONTRIBUTING.md`

### Files to Significantly Modify
- `cmd/crawl/main.go` - Use app lifecycle
- `internal/cli/root.go` - Use config system
- `internal/cli/get.go` - Use config & DI
- `internal/cache/cache.go` - Remove global, add interface
- `internal/ratelimit/limiter.go` - Remove global, add interface
- `internal/engine/static.go` - Use injected dependencies
- `internal/engine/dynamic.go` - Use injected dependencies
- `internal/engine/browser_pool.go` - Use config & FindChrome

### Files to Reorganize (MOVE)
- Engine internals → `internal/engine/static/` and `internal/engine/dynamic/`
- CLI commands → `internal/cli/commands/`

### Files to Keep As-Is
- `pkg/models/models.go` - Core data models
- `internal/auth/` - Authentication logic
- `internal/downloader/` - Download logic
- `internal/retry/` - Retry logic

---

## Success Criteria

1. ✅ Zero global variables in codebase
2. ✅ All configuration from single source
3. ✅ All resources properly lifecycle managed
4. ✅ 100% cross-platform browser detection
5. ✅ All dependencies injectable
6. ✅ Test coverage ≥ 80%
7. ✅ Each package has clear single responsibility
8. ✅ No circular dependencies
9. ✅ Complete documentation for each package
10. ✅ All previous functionality preserved

---

## Next Steps

1. **Review this plan** - Does it align with your vision?
2. **Prioritize** - Which phases are most important?
3. **Start with Phase 1** - Foundation must be solid first
4. **Branch strategy** - Create feature branch for refactoring
5. **Review incrementally** - Test and review each phase
6. **Benchmark** - Compare performance before/after

---

## Questions & Considerations

- Do you want YAML or TOML config files?
- Should we add a `.env` file example?
- Any existing performance requirements to maintain?
- What's the minimum Go version to support?
- Do you want Docker support for testing?

---

**Created**: December 21, 2025  
**Status**: Ready for Review  
**Estimated Total Effort**: 3-4 weeks (depending on parallelization)
