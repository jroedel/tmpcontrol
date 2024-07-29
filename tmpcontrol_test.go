package tmpcontrol

import (
	"github.com/jroedel/tmpcontrol/business/busconfiggopher"
	"testing"
)

func TestConfigGopher_HasError(t *testing.T) {
	cg := busconfiggopher.ConfigGopher{}
	err := cg.HasError()
	if err == nil {
		t.Error("expected error since we didn't specify server info nor local file")
	}

	cg2 := busconfiggopher.ConfigGopher{ServerRoot: "localhost"}
	err = cg2.HasError()
	if err == nil {
		t.Error("expected error since we specified server root, but not client identifier")
	}

	cg3 := busconfiggopher.ConfigGopher{ServerRoot: "localhost", ClientId: "test-id"}
	err = cg3.HasError()
	if err != nil {
		t.Error("expected no error since we specified server root and client identifier")
	}

	cg4 := busconfiggopher.ConfigGopher{ServerRoot: "localhost", ClientId: "test id"}
	err = cg4.HasError()
	if err == nil {
		t.Error("expected error since spaces in the ClientId are prohibited")
	}
}
