package tekton

import (
	"errors"
	"testing"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

// gvrForErr is a placeholder GroupVersionResource used when synthesising
// k8s Status errors for tests. The actual group/resource doesn't matter
// for the `IsNotFound`-style predicates used by mapK8sError.
var gvrForErr = schema.GroupResource{Group: "tekton.dev", Resource: "pipelineruns"}

func TestMapK8sError_Nil(t *testing.T) {
	if err := mapK8sError(nil); err != nil {
		t.Fatalf("nil should return nil, got %v", err)
	}
}

func TestMapK8sError_NotFound(t *testing.T) {
	src := k8serrors.NewNotFound(gvrForErr, "x")
	if err := mapK8sError(src); !errors.Is(err, engine.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestMapK8sError_Unauthorized(t *testing.T) {
	src := k8serrors.NewUnauthorized("no")
	if err := mapK8sError(src); !errors.Is(err, engine.ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
}

func TestMapK8sError_Forbidden(t *testing.T) {
	src := k8serrors.NewForbidden(gvrForErr, "x", errors.New("denied"))
	if err := mapK8sError(src); !errors.Is(err, engine.ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized for 403, got %v", err)
	}
}

func TestMapK8sError_BadRequest(t *testing.T) {
	src := k8serrors.NewBadRequest("bad")
	if err := mapK8sError(src); !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestMapK8sError_Invalid(t *testing.T) {
	src := k8serrors.NewInvalid(schema.GroupKind{Group: "tekton.dev", Kind: "PipelineRun"}, "x", nil)
	if err := mapK8sError(src); !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestMapK8sError_Conflict(t *testing.T) {
	src := k8serrors.NewConflict(gvrForErr, "x", errors.New("version mismatch"))
	if err := mapK8sError(src); !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestMapK8sError_AlreadyExists(t *testing.T) {
	src := k8serrors.NewAlreadyExists(gvrForErr, "x")
	if err := mapK8sError(src); !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestMapK8sError_ServerTimeout(t *testing.T) {
	src := k8serrors.NewServerTimeout(gvrForErr, "get", 5)
	if err := mapK8sError(src); !errors.Is(err, engine.ErrUnavailable) {
		t.Fatalf("expected ErrUnavailable, got %v", err)
	}
}

func TestMapK8sError_ServiceUnavailable(t *testing.T) {
	src := k8serrors.NewServiceUnavailable("down")
	if err := mapK8sError(src); !errors.Is(err, engine.ErrUnavailable) {
		t.Fatalf("expected ErrUnavailable, got %v", err)
	}
}

func TestMapK8sError_InternalError(t *testing.T) {
	src := k8serrors.NewInternalError(errors.New("boom"))
	if err := mapK8sError(src); !errors.Is(err, engine.ErrUnavailable) {
		t.Fatalf("expected ErrUnavailable, got %v", err)
	}
}

func TestMapK8sError_TooManyRequests(t *testing.T) {
	src := k8serrors.NewTooManyRequests("slow down", 10)
	if err := mapK8sError(src); !errors.Is(err, engine.ErrUnavailable) {
		t.Fatalf("expected ErrUnavailable, got %v", err)
	}
}

func TestMapK8sError_GenericFallback(t *testing.T) {
	// Non-status errors (e.g. dial error, DNS failure) fall through to
	// ErrUnavailable.
	src := errors.New("dial tcp: connection refused")
	if err := mapK8sError(src); !errors.Is(err, engine.ErrUnavailable) {
		t.Fatalf("expected ErrUnavailable, got %v", err)
	}
}

func TestMapK8sError_PreservesUnderlyingError(t *testing.T) {
	src := k8serrors.NewNotFound(gvrForErr, "my-pipeline")
	mapped := mapK8sError(src)
	// The original k8s error should remain unwrappable alongside the sentinel.
	if !errors.Is(mapped, src) {
		t.Fatalf("mapped error should still wrap the original: %v", mapped)
	}
	if !errors.Is(mapped, engine.ErrNotFound) {
		t.Fatalf("mapped should ALSO match ErrNotFound")
	}
}

// Ensure metav1 is imported (tests reference schema types indirectly through
// k8serrors constructors; the blank identifier here guards against the
// import being optimised away in future refactors).
var _ = metav1.ObjectMeta{}

func TestClusterResolver_RequireResolver_Nil(t *testing.T) {
	if err := requireResolver(nil); !errors.Is(err, engine.ErrUnavailable) {
		t.Fatalf("nil resolver should map to ErrUnavailable, got %v", err)
	}
}
