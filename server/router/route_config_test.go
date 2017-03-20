package router

import (
	"testing"

	"github.com/cosminrentea/gobbler/protocol"
	"github.com/stretchr/testify/assert"
)

type routeConfig struct {
	path   string
	fields map[string]string
}

func TestRouteConfig_Equal(t *testing.T) {
	a := assert.New(t)

	testcases := map[string]struct {
		// first route definition
		first routeConfig

		// second route definition
		second routeConfig

		Matcher Matcher

		// keys to pass on matching
		keys []string

		// expected result
		result bool
	}{
		"full equal": {
			first: routeConfig{
				path: "/path",
				fields: map[string]string{
					"field1": "value1",
					"field2": "value2",
				},
			},
			second: routeConfig{
				path: "/path",
				fields: map[string]string{
					"field1": "value1",
					"field2": "value2",
				},
			},
			result: true,
		},

		"full equal with matcher": {
			first: routeConfig{
				path: "/path",
				fields: map[string]string{
					"field1": "value1",
					"field2": "value2",
				},
			},
			second: routeConfig{
				path: "/path",
				fields: map[string]string{
					"field1": "value1",
					"field2": "value2",
				},
			},
			Matcher: func(config RouteConfig, other RouteConfig, keys ...string) bool {
				return config.Path == other.Path
			},
			result: true,
		},

		"make sure matcher is called": {
			first: routeConfig{
				path: "/path",
				fields: map[string]string{
					"field1": "value1",
					"field2": "value2",
				},
			},
			second: routeConfig{
				path: "/incorrect-path",
				fields: map[string]string{
					"field1": "value1",
					"field2": "value2",
				},
			},
			Matcher: func(config RouteConfig, other RouteConfig, keys ...string) bool {
				return true
			},
			result: true,
		},

		"partial match": {
			first: routeConfig{
				path: "/path",
				fields: map[string]string{
					"field1": "value1",
					"field2": "value2",
				},
			},
			second: routeConfig{
				path: "/path",
				fields: map[string]string{
					"field1": "value1",
					"field3": "value3",
				},
			},
			keys:   []string{"field1"},
			result: true,
		},

		"unequal path with keys": {
			first: routeConfig{
				path: "/path",
				fields: map[string]string{
					"field1": "value1",
					"field2": "value2",
				},
			},
			second: routeConfig{
				path: "/different-path",
				fields: map[string]string{
					"field1": "value1",
					"field3": "value3",
				},
			},
			keys:   []string{"field1"},
			result: false,
		},
	}

	for name, c := range testcases {
		first := RouteConfig{
			Path:        protocol.Path(c.first.path),
			RouteParams: RouteParams(c.first.fields),
			Matcher:     c.Matcher,
		}
		second := RouteConfig{
			Path:        protocol.Path(c.second.path),
			RouteParams: RouteParams(c.second.fields),
			Matcher:     c.Matcher,
		}
		a.Equal(c.result, first.Equal(second, c.keys...), "Failed forward check for case: "+name)
		a.Equal(c.result, second.Equal(first, c.keys...), "Failed backwards check for case: "+name)
	}
}

func TestRouteConfig_messageFilter(t *testing.T) {
	a := assert.New(t)

	routeConfig := RouteConfig{
		RouteParams: RouteParams{
			"field1": "value1",
			"field2": "value2",
		},
	}

	testcases := map[string]struct {
		// filters on the message
		filters map[string]string

		// expected result
		result bool
	}{
		"no filter": {
			filters: nil,
			result:  true,
		},
		"partial filter": {
			filters: map[string]string{
				"field1": "value1",
			},
			result: true,
		},
		"full filter": {
			filters: map[string]string{
				"field1": "value1",
				"field2": "value2",
			},
			result: true,
		},
		"one invalid filter": {
			filters: map[string]string{
				"field1": "value1",
				"field2": "value3",
			},
			result: false,
		},
		"both invalid": {
			filters: map[string]string{
				"field1": "value3",
				"field2": "value4",
			},
			result: false,
		},
		"partial invalid": {
			filters: map[string]string{
				"field2": "value4",
			},
			result: false,
		},
	}

	for name, c := range testcases {
		m := &protocol.Message{Filters: c.filters}
		a.Equal(c.result, routeConfig.messageFilter(m), "Failed filter: "+name)
	}
}
