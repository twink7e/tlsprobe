package dnsprovider

import (
	"testing"
)

func TestRefreshResources(t *testing.T) {
	domain1 := NewDomain()
	domain1.Add("xiaoshuo.com", "1.1.1.1", RecordTypeA)
	domain1.Add("1.xiaoshuo.com", "1.1.1.1", RecordTypeA)
	domain1.Add("2.xiaoshuo.com", "1.1.1.1", RecordTypeA)
	domain1.Add("1.1.xiaoshuo.com", "1.1.1.1", RecordTypeA)

	domain2 := NewDomain()
	domain2.Add("xiaoshuo.com", "1.1.1.1", RecordTypeA)
	domain2.Add("1.xiaoshuo.com", "1.1.1.1", RecordTypeA)
	domain2.Add("2.xiaoshuo.com", "1.1.1.1", RecordTypeA)
	//domain2.Add("1.1.xiaoshuo.com", "1.1.1.1", RecordTypeA)

	records := GetShouldRefreshRecords(domain1.Records, domain2.Records)
	if len(records) != 1 {
		t.Fatalf("records length should be 1, but now is: %d", len(records))
	}
}
