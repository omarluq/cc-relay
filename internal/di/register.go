package di

import "github.com/samber/do/v2"

// RegisterSingletons registers all service providers as singletons.
// Services are registered in dependency order:
// 1. Config (no dependencies)
// 2. Logger (depends on Config)
// 3. Cache (depends on Config)
// 4. Providers (depends on Config)
// 5. KeyPool (depends on Config) - primary provider only
// 6. KeyPoolMap (depends on Config) - all providers
// 7. Router (depends on Config)
// 8. HealthTracker (depends on Config, Logger)
// 9. Checker (depends on HealthTracker, Config, Logger)
// 10. ProviderInfo (depends on Config, Providers, HealthTracker)
// 11. SignatureCache (depends on Cache)
// 12. Concurrency (depends on Config) - global request limiter
// 13. Handler (depends on all above services)
// 14. Server (depends on Handler, Config).
func RegisterSingletons(injector do.Injector) {
	do.Provide(injector, NewConfig)
	do.Provide(injector, NewLogger)
	do.Provide(injector, NewCache)
	do.Provide(injector, NewProviderMap)
	do.Provide(injector, NewKeyPool)
	do.Provide(injector, NewKeyPoolMap)
	do.Provide(injector, NewRouter)
	do.Provide(injector, NewHealthTracker)
	do.Provide(injector, NewChecker)
	do.Provide(injector, NewProviderInfo)
	do.Provide(injector, NewSignatureCache)
	do.Provide(injector, NewConcurrencyService)
	do.Provide(injector, NewProxyHandler)
	do.Provide(injector, NewHTTPServer)
}
