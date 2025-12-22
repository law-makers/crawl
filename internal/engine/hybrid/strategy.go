// internal/engine/hybrid/strategy.go
package hybrid

// Strategy represents the scraping strategy to use
type Strategy int

const (
	// StrategyStatic uses only static HTML scraping
	StrategyStatic Strategy = iota

	// StrategyHybrid uses static scraping with JS execution
	StrategyHybrid

	// StrategyDynamic uses full browser rendering
	StrategyDynamic
)

// String returns the string representation of the strategy
func (s Strategy) String() string {
	switch s {
	case StrategyStatic:
		return "Static"
	case StrategyHybrid:
		return "Hybrid"
	case StrategyDynamic:
		return "Dynamic"
	default:
		return "Unknown"
	}
}

// DetermineStrategy decides which strategy to use based on page characteristics
func DetermineStrategy(html string, scriptCount int) Strategy {
	// If no scripts, use static
	if scriptCount == 0 {
		return StrategyStatic
	}

	// Check if needs full browser
	if NeedsJavaScript(html, scriptCount) {
		return StrategyDynamic
	}

	// If has scripts but not a SPA, use hybrid
	if scriptCount > 0 {
		return StrategyHybrid
	}

	return StrategyStatic
}
