package dnsprovider

import (
	"errors"
	"strings"
	"sync"
)

type RecordType string

const (
	RecordTypeA     RecordType = "A"
	RecordTypeCName RecordType = "CNAME"
	RecordTypeNS    RecordType = "NS"
)

// Domain is .
type Domain struct {
	Records map[string]*Record
	mux     *sync.RWMutex
}

func NewDomain() *Domain {
	return &Domain{
		mux: new(sync.RWMutex),
	}
}

func (d *Domain) Add(fqdn string, recordValue string, recordType RecordType) (*Record, error) {
	s := strings.Split(fqdn, ".")
	sLength := len(s)
	if sLength < 1 {
		return nil, errors.New("fqdn too short")
	}
	if s[sLength-1] == "" {
		s = s[:sLength]
	}
	d.mux.Lock()
	defer d.mux.Unlock()
	if d.Records == nil {
		d.Records = make(map[string]*Record)
	}

	i := sLength - 1
	rcs := d.Records
	var rc *Record
	var exists bool
	for ; i > -1; i-- {
		name := s[i]
		rc, exists = rcs[name]
		if !exists {
			rc = &Record{
				Domain:   strings.Join(s[i+1:], "."),
				Record:   name,
				Children: make(map[string]*Record),
			}
			rcs[name] = rc
		}
		rcs = rc.Children
	}
	if rc.Value == nil {
		rc.Value = make(RecordValue, 1)
		rc.Value[0] = recordValue
	} else {
		rc.Value = append(rc.Value, recordValue)
	}
	rc.Type = recordType
	return rc, nil
}

func (d *Domain) Search(fqdn string) *Record {
	d.mux.RLock()
	defer d.mux.RUnlock()
	if d.Records == nil {
		return nil
	}
	s := strings.Split(fqdn, ".")
	sLength := len(s)
	if sLength < 1 {
		return nil
	}
	i := sLength - 1
	rcs := d.Records
	var rc *Record
	var exists bool
	for ; i > -1; i-- {
		name := s[i]
		rc, exists = rcs[name]
		if !exists {
			return nil
		}
		rcs = rc.Children
	}
	return rc
}

type RecordValue []string

type Record struct {
	Domain   string
	Type     RecordType
	Record   string
	Value    RecordValue
	Children map[string]*Record
}

func GetFQDN(record *Record) string {
	if record.Record == "@" {
		return record.Domain
	}
	switch record.Record {
	case "@":
		return record.Domain
	case "*":
		return "fakename2222replacewildcard" + "." + record.Domain
	}
	return record.Record + "." + record.Domain
}
