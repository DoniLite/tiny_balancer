package core

const (
	MogolyRatelimiter MiddleWareName = "mogoly:ratelimiter"
)

var MiddlewaresList MiddlewareSets = MiddlewareSets{
	MogolyRatelimiter: struct {
		Fn   MogolyMiddleware
		Conf any
	}{
		Fn:   RateLimiterMiddleware,
		Conf: RateLimitMiddlewareConfig{},
	},
}
