package dnsprovider

import (
	"fmt"
	"sync"
	"testing"
)

func TestDomainAdd(t *testing.T) {
	domain := Domain{
		mux: new(sync.RWMutex),
	}

	record, err := domain.Add("1.xiaoshuo.baidu.com", "1.1.1.1", RecordTypeA)
	if err != nil {
		t.Fatal(err)
	}
	if domain.Records["com"].Children["baidu"].Children["xiaoshuo"].Children["1"].Value != "1.1.1.1" {
		t.FailNow()
	}
	if record.Domain != "xiaoshuo.baidu.com" {
		t.Fatalf("domain test error, should not be: %s", record.Domain)
	}
	if domain.Records["com"].Domain != "" {
		t.Fatalf("domain test error, should not be: %s", domain.Records["com"].Domain)
	}
	fmt.Println(domain.Records["com"].Children["baidu"].Children["xiaoshuo"].Children["1"].Value)
}

func TestDomainSearch(t *testing.T) {
	domain := Domain{
		mux: new(sync.RWMutex),
	}

	domain.Add("1.xiaoshuo.baidu.com", "1.1.1.1", RecordTypeA)
	record := domain.Search("xiaoshuo.baidu.com")
	if record == nil {
		t.Fatalf("search failed")
	}
	if record.Children["1"].Value != "1.1.1.1" {
		t.Fatalf("wrong value for record")
	}
	fmt.Println(record.Children["1"].Value)
}
