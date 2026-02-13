package graph

import "testing"

func TestDecodeODataPage(t *testing.T) {
	body := []byte(`{
		"value": [{"id":"1"},{"id":"2"}],
		"@odata.nextLink":" https://graph.microsoft.com/v1.0/me/messages?$skiptoken=abc "
	}`)

	items, next, err := DecodeODataPage(body)
	if err != nil {
		t.Fatalf("DecodeODataPage failed: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected two items, got %d", len(items))
	}
	if next != "https://graph.microsoft.com/v1.0/me/messages?$skiptoken=abc" {
		t.Fatalf("unexpected next link: %q", next)
	}
}

func TestDecodeODataPageInvalidJSON(t *testing.T) {
	if _, _, err := DecodeODataPage([]byte(`{`)); err == nil {
		t.Fatal("expected decode error")
	}
}
