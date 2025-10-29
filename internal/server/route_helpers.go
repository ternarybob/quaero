package server

import (
	"net/http"
	"strings"
)

// RouteHandler is a function type for HTTP handlers
type RouteHandler func(http.ResponseWriter, *http.Request)

// MethodRouter maps HTTP methods to handlers
type MethodRouter map[string]RouteHandler

// RouteByMethod routes requests based on HTTP method with standardized error handling
func RouteByMethod(w http.ResponseWriter, r *http.Request, routes MethodRouter) {
	handler, ok := routes[r.Method]
	if !ok {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	handler(w, r)
}

// RouteCRUD is a convenience function for standard CRUD operations (GET, POST, PUT, DELETE)
func RouteCRUD(w http.ResponseWriter, r *http.Request, get, post, put, delete RouteHandler) {
	routes := make(MethodRouter)
	if get != nil {
		routes["GET"] = get
	}
	if post != nil {
		routes["POST"] = post
	}
	if put != nil {
		routes["PUT"] = put
	}
	if delete != nil {
		routes["DELETE"] = delete
	}
	RouteByMethod(w, r, routes)
}

// PathSuffixRouter checks if path ends with a specific suffix and routes to handler
type PathSuffixRouter struct {
	Suffix  string
	Handler RouteHandler
}

// RouteByPathSuffix routes requests based on path suffix
// Returns true if a route was matched and handled
func RouteByPathSuffix(w http.ResponseWriter, r *http.Request, prefix string, routes []PathSuffixRouter) bool {
	path := r.URL.Path
	if len(path) <= len(prefix) {
		return false
	}

	pathSuffix := path[len(prefix):]
	for _, route := range routes {
		if strings.HasSuffix(pathSuffix, route.Suffix) || pathSuffix == route.Suffix {
			route.Handler(w, r)
			return true
		}
	}
	return false
}

// RouteResourceCollection handles standard list + create pattern
// GET -> list, POST -> create
func RouteResourceCollection(w http.ResponseWriter, r *http.Request, list, create RouteHandler) {
	RouteCRUD(w, r, list, create, nil, nil)
}

// RouteResourceItem handles standard get + update + delete pattern
// GET -> get, PUT -> update, DELETE -> delete
func RouteResourceItem(w http.ResponseWriter, r *http.Request, get, update, delete RouteHandler) {
	RouteCRUD(w, r, get, nil, update, delete)
}
