package headers

import (
	"reflect"
	"testing"
)

func TestParseHeaders(t *testing.T) {
	in := []string{"User-Agent: Bot", "Accept: text/html", "BadHeader"}
	out := ParseHeaders(in)
	expected := map[string]string{"User-Agent": "Bot", "Accept": "text/html"}
	if !reflect.DeepEqual(out, expected) {
		t.Fatalf("unexpected parse result: %#v", out)
	}
}
