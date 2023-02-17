package aliyun

import (
	"context"
	"errors"
	dnscli "github.com/alibabacloud-go/alidns-20150109/v4/client"
	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	"github.com/rs/zerolog/log"
	"time"
	"tlsprobe/autodiscover"
	"tlsprobe/common/creator"
	"tlsprobe/dnsprovider"
)

func Creator(ctx context.Context, cfg *autodiscover.Config, creator creator.Creator) (autodiscover.AutoDiscover, error) {
	ak, exists := cfg.Options["accessKeyId"]
	if !exists {
		return nil, errors.New("accessKeyId not exists in options")
	}
	sk, exists := cfg.Options["accessKeySecret"]
	if !exists {
		return nil, errors.New("accessKeySecret not exists in options")
	}
	openapiConfig := openapi.Config{
		AccessKeyId:     &ak,
		AccessKeySecret: &sk,
	}
	dnsCli, err := dnscli.NewClient(&openapiConfig)
	if err != nil {
		return nil, err
	}
	if err := HealthChecker(dnsCli); err != nil {
		return nil, err
	}
	return NewDNSProvider(ctx, cfg.Name, dnsCli, cfg, creator), nil
}

func HealthChecker(client *dnscli.Client) error {
	var pageSize int64 = 1
	req := dnscli.DescribeDomainsRequest{}
	req.SetPageSize(pageSize)
	if _, err := client.DescribeDomains(&req); err != nil {
		return err
	}
	return nil
}

type DNSProvider struct {
	creator.Creator
	Name        string
	client      *dnscli.Client
	ctx         context.Context
	cancelFunc  context.CancelFunc
	domains     *dnsprovider.Domain
	lastDomains *dnsprovider.Domain
	cfg         *autodiscover.Config
}

func NewDNSProvider(parentCtx context.Context, name string, cli *dnscli.Client, cfg *autodiscover.Config, creator creator.Creator) *DNSProvider {
	if parentCtx == nil {
		parentCtx = context.Background()
	}
	ctx, cancel := context.WithCancel(parentCtx)
	return &DNSProvider{
		Name:       name,
		client:     cli,
		ctx:        ctx,
		cancelFunc: cancel,
		domains:    dnsprovider.NewDomain(),
		cfg:        cfg,
		Creator:    creator,
	}
}

func (d *DNSProvider) AliRecordToRecord(fqdn string, r *dnscli.DescribeDomainRecordsResponseBodyDomainRecordsRecord) (*dnsprovider.Record, error) {
	if r == nil {
		return nil, errors.New("DescribeDomainRecordsResponseBodyDomainRecordsRecord should not be nil")
	}
	var recordType dnsprovider.RecordType
	switch *r.Type {
	case string(dnsprovider.RecordTypeA):
		recordType = dnsprovider.RecordTypeA
	case string(dnsprovider.RecordTypeCName):
		recordType = dnsprovider.RecordTypeCName
	default:
		return nil, errors.New("not support RecordType:" + *r.Type)
	}
	return d.domains.Add(fqdn, *r.Value, recordType)
}

func (d *DNSProvider) getDomains() {
	var pageSize int64 = 10
	var pageNum int64 = 1
	// switch domains to lastDomains and reset domains.
	d.lastDomains = d.domains
	d.domains = dnsprovider.NewDomain()
	// start to fetch domains.
	for ; ; pageNum++ {
		// handle stop event.
		select {
		case <-d.ctx.Done():
			return
		default:
		}
		// limit request.
		time.Sleep(time.Millisecond * 150)
		req := dnscli.DescribeDomainsRequest{
			PageSize:   &pageSize,
			PageNumber: &pageNum,
		}
		var resp *dnscli.DescribeDomainsResponse
		var err error
		// support retry.
		for retryTimes := 0; retryTimes < 3; retryTimes++ {
			resp, err = d.client.DescribeDomains(&req)
			if err != nil {
				// sleep 1s when got an error.
				log.Warn().Msgf("DNSProvider %s starting get domains got an error: %s", d.Name, err)
				time.Sleep(time.Second * 1)
				continue
			}
		}
		// out of retryTimes break.
		if err != nil {
			log.Warn().Msgf("DNSProvider %s starting get domains failed: out of retry times", d.Name)
			break
		}
		log.Debug().Msgf("DNSProvider %s got domains: %d", d.Name, len(resp.Body.Domains.Domain))
		for _, rawDomain := range resp.Body.Domains.Domain {
			//if _, exists := d.domains[domain.Name]; !exists {
			//	d.DeleteDomain(domain.Name)
			//}
			d.getDomainRecords(*rawDomain.DomainName)
		}
		if len(resp.Body.Domains.Domain) < int(pageSize) {
			break
		}

	}
	dnsprovider.RefreshResources(d.lastDomains.Records, d.domains.Records)
}

func (d *DNSProvider) getDomainRecords(domainName string) {
	var pageSize int64 = 20
	var pageNum int64 = 1

	for ; ; pageNum++ {
		// handle stop event.
		select {
		case <-d.ctx.Done():
			return
		default:
		}
		// limit request.
		time.Sleep(time.Millisecond * 150)
		req := dnscli.DescribeDomainRecordsRequest{
			PageSize:   &pageSize,
			PageNumber: &pageNum,
			DomainName: &domainName,
		}
		var resp *dnscli.DescribeDomainRecordsResponse
		var err error
		// support retry.
		for retryTimes := 0; retryTimes < 3; retryTimes++ {
			resp, err = d.client.DescribeDomainRecords(&req)
			if err != nil {
				// sleep 1s when got an error.
				log.Warn().Msgf("DNSProvider %s starting get domainName records got an error: %s", d.Name, err)
				time.Sleep(time.Second * 1)
				continue
			}
		}
		// out of retryTimes break.
		if err != nil {
			log.Warn().Msgf("DNSProvider %s starting get domainName records failed: out of retry times", d.Name)
			break
		}
		log.Debug().Msgf("DNSProvider %s got domainName records: %d", d.Name, len(resp.Body.DomainRecords.Record))
		for _, rawRecord := range resp.Body.DomainRecords.Record {
			//if _, exists := domain.Records[*rawRecord.RR]; !exists {
			//	d.DeleteRecord(domain.Name, *rawRecord.RR)
			//}
			fqdn := *rawRecord.RR + "." + domainName
			record, err := d.AliRecordToRecord(fqdn, rawRecord)
			if err != nil {
				log.Debug().Msgf("aliyun dnsprovider add to domains error: %v", err)
				continue
			}
			dnsprovider.MakeHostScanner(d.ctx, record, d)
		}
		if len(resp.Body.DomainRecords.Record) < int(pageSize) {
			break
		}
	}
	return
}

func (d *DNSProvider) DeleteRecord(domainName string, recordName string) {

	// delete host scanner.
	return
}

func (d *DNSProvider) Start() error {
	go func() {
		d.getDomains()
		for {
			select {
			case <-time.After(time.Minute * 10):
				d.getDomains()
			case <-d.ctx.Done():
				return
			}
		}
	}()

	return nil
}

func (d *DNSProvider) Stop() error {
	dnsprovider.RefreshResources(d.domains.Records, nil)
	log.Info().Msgf("shutdown DNSProvider :%s", d.Describe())

	d.cancelFunc()
	return nil
}

func (d *DNSProvider) Config() *autodiscover.Config {
	return d.cfg
}

func (d *DNSProvider) Key() string {
	return d.Describe()
}

func (d *DNSProvider) GetCreator() creator.Creator {
	return d.Creator
}

func (d *DNSProvider) Describe() string {
	return "DNSProvider: " + d.Name
}
