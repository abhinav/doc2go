module example.com/replacepkg

go 1.21

require (
	go.uber.org/zap v1.27.1
	github.com/pkg/errors v0.9.1
	golang.org/x/text v0.14.0
)

// Replace with a different version - should use v1.26.0 instead of v1.27.1
replace go.uber.org/zap => go.uber.org/zap v1.26.0

// Local path replacement - should be skipped/ignored
// (we don't actually create this directory, testing error handling)
replace github.com/pkg/errors => ./vendor/errors
