package boardapi

import "net/http"

// ParseListMetaForTest exposes parseListMeta for external package tests
// (boardapi_test) without widening the public API surface.
func ParseListMetaForTest(h http.Header) ListMeta { return parseListMeta(h) }

// ParseItemMetaForTest exposes parseItemMeta for external package tests.
func ParseItemMetaForTest(h http.Header) ItemMeta { return parseItemMeta(h) }
