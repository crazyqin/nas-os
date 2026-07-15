package web

import (
	"reflect"
	"sort"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestLegacyStorageRouteSnapshot(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	registerLegacyStorageRoutes(router.Group("/api/v1"), &Server{})

	got := make([]string, 0, len(router.Routes()))
	for _, route := range router.Routes() {
		got = append(got, route.Method+" "+route.Path)
	}
	sort.Strings(got)
	want := []string{
		"DELETE /api/v1/volumes/:name",
		"DELETE /api/v1/volumes/:name/devices/:device",
		"DELETE /api/v1/volumes/:name/snapshots/:snapshot",
		"DELETE /api/v1/volumes/:name/subvolumes/:subvol",
		"GET /api/v1/raid-configs",
		"GET /api/v1/volumes",
		"GET /api/v1/volumes/:name",
		"GET /api/v1/volumes/:name/balance",
		"GET /api/v1/volumes/:name/devices",
		"GET /api/v1/volumes/:name/scrub",
		"GET /api/v1/volumes/:name/subvolumes",
		"GET /api/v1/volumes/:name/subvolumes/:subvol",
		"GET /api/v1/volumes/:name/snapshots",
		"GET /api/v1/volumes/:name/usage",
		"POST /api/v1/volumes",
		"POST /api/v1/volumes/:name/balance",
		"POST /api/v1/volumes/:name/convert",
		"POST /api/v1/volumes/:name/devices",
		"POST /api/v1/volumes/:name/mount",
		"POST /api/v1/volumes/:name/scrub",
		"POST /api/v1/volumes/:name/snapshots",
		"POST /api/v1/volumes/:name/snapshots/:snapshot/restore",
		"POST /api/v1/volumes/:name/subvolumes",
		"POST /api/v1/volumes/:name/unmount",
		"PUT /api/v1/volumes/:name/subvolumes/:subvol/readonly",
	}
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("legacy storage routes changed\ngot:  %v\nwant: %v", got, want)
	}
}
