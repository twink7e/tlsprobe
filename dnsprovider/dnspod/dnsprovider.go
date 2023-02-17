package dnspod

import (
	"context"
	"errors"
	"fmt"
	"github.com/rs/zerolog/log"
	dnspodcommon "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	dnspod "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/dnspod/v20210323"
	"tlsprobe/autodiscover"
	"tlsprobe/common/creator"
	"tlsprobe/dnsprovider"
	"time"
)

func Creator(ctx context.Context, cfg *autodiscover.Config, creator creator.Creator) (autodiscover.AutoDiscover, error) {
	si, exists := cfg.Options["secretId"]
	if !exists {
		return nil, errors.New("SecretId not exists in options")
	}
	sk, exists := cfg.Options["secretKey"]
	if !exists {
		return nil, errors.New("SecretKey not exists in options")
	}
	credential := dnspodcommon.NewCredential(si, sk)

	client, err := dnspod.NewClient(credential, "", profile.NewClientProfile())
	if err != nil {
		return nil, err
	}
	if err := HealthChecker(client); err != nil {
		return nil, fmt.Errorf("DNSPod Creator create dnsprovider failed, healthChecker err: %w", err)
	}
	p := NewDNSProvider(ctx, cfg.Name, client, cfg, creator)
	return p, nil
}

func HealthChecker(client *dnspod.Client) error {
	var pageSize int64 = 1
	request := dnspod.NewDescribeDomainListRequest()
	request.Limit = &pageSize
	_, err := client.DescribeDomainList(request)
	return err
}

type DNSProvider struct {
	creator.Creator
	Name        string
	client      *dnspod.Client
	ctx         context.Context
	cancelFunc  context.CancelFunc
	domains     *dnsprovider.Domain
	lastDomains *dnsprovider.Domain
	cfg         *autodiscover.Config
}

func NewDNSProvider(parentCtx context.Context, name string, cli *dnspod.Client, cfg *autodiscover.Config, creator creator.Creator) *DNSProvider {
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

func (p *DNSProvider) AddToDomains(fqdn string, r *dnspod.RecordListItem) (*dnsprovider.Record, error) {
	if r == nil {
		return nil, errors.New("RecordListItem cannot be nil")
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
	return p.domains.Add(fqdn, *r.Value, recordType)
}

func (p *DNSProvider) shouldStop() bool {
	// handle stop event.
	select {
	case <-p.ctx.Done():
		return true
	default:
		return false
	}
}

func (p *DNSProvider) GetDomains() {
	var pageSize int64 = 10
	var pageNum int64 = 1
	// switch domains to lastDomains and reset domains.
	p.lastDomains = p.domains
	p.domains = dnsprovider.NewDomain()
	// start to fetch domains.
	for ; ; pageNum++ {
		// handle stop event.
		if p.shouldStop() {
			break
		}

		offset := (pageNum - 1) * pageNum
		nextOffset := pageNum * pageSize
		req := dnspod.NewDescribeDomainListRequest()
		req.Limit = &pageSize
		req.Offset = &offset

		// support retry.
		var resp *dnspod.DescribeDomainListResponse
		var err error
		for retryTimes := 0; retryTimes < 3; retryTimes++ {
			resp, err = p.client.DescribeDomainList(req)
			if err != nil {
				// sleep 1s when got an error.
				log.Warn().Msgf("DNSProvider %s starting get domains got an error: %s", p.Name, err)
				time.Sleep(time.Second * 1)
				continue
			}
		}
		// out of retryTimes break.
		if err != nil {
			log.Warn().Msgf("DNSProvider %s starting get domains failed: out of retry times", p.Name)
			break
		}
		log.Debug().Msgf("DNSProvider %s got domains: %d", p.Name, len(resp.Response.DomainList))
		for _, rawDomain := range resp.Response.DomainList {
			if *rawDomain.Status != "ENABLE" {
				continue
			}
			p.getDomainRecords(*rawDomain.Name)
		}
		log.Debug().Msgf(
			"get domains total count: %d, offset: %d, next offset: %d",
			*resp.Response.DomainCountInfo.DomainTotal,
			offset,
			nextOffset,
		)
		if len(resp.Response.DomainList) < int(pageSize) || uint64(nextOffset) >= *resp.Response.DomainCountInfo.DomainTotal {
			break
		}
	}
	dnsprovider.RefreshResources(p.lastDomains.Records, p.domains.Records)
}

func (p *DNSProvider) getDomainRecords(domainName string) {
	var pageSize uint64 = 10
	var pageNum uint64 = 1
	for ; ; pageNum++ {
		// handle stop event.
		if p.shouldStop() {
			break
		}

		offset := (pageNum - 1) * pageSize
		nextOffset := pageNum * pageSize
		req := dnspod.NewDescribeRecordListRequest()
		req.Limit = &pageSize
		req.Offset = &offset
		req.Domain = &domainName

		// support retry.
		var resp *dnspod.DescribeRecordListResponse
		var err error
		for retryTimes := 0; retryTimes < 3; retryTimes++ {
			resp, err = p.client.DescribeRecordList(req)
			if err != nil {
				// sleep 1s when got an error.
				log.Warn().Msgf("DNSProvider %s starting get domains got an error: %s", p.Name, err)
				time.Sleep(time.Second * 1)
				continue
			}
		}
		// out of retryTimes break.
		if err != nil {
			log.Warn().Msgf("DNSProvider %s starting get domains failed: out of retry times", p.Name)
			break
		}
		log.Debug().Msgf("DNSProvider %s got domain %s records: %d", p.Name, domainName, len(resp.Response.RecordList))
		for _, rawRecord := range resp.Response.RecordList {
			if *rawRecord.Status != "ENABLE" {
				continue
			}
			fqdn := *rawRecord.Name + "." + domainName
			record, err := p.AddToDomains(fqdn, rawRecord)
			if err != nil {
				log.Debug().Msgf("add to domains error: %v", err)
				continue
			}
			dnsprovider.MakeHostScanner(p.ctx, record, p)
		}
		log.Debug().Msgf(
			"get domain %s records total count: %d, offset: %d, next offset: %d",
			domainName,
			*resp.Response.RecordCountInfo.TotalCount,
			offset,
			nextOffset,
		)
		if len(resp.Response.RecordList) < int(pageSize) || nextOffset >= *resp.Response.RecordCountInfo.TotalCount {
			break
		}
	}
}

func (p *DNSProvider) Start() error {
	go func() {
		p.GetDomains()
		for {
			select {
			case <-p.ctx.Done():
				return
			case <-time.After(1 * time.Minute):
				p.GetDomains()
			}
		}
	}()
	return nil
}

func (p *DNSProvider) Stop() error {
	dnsprovider.RefreshResources(p.domains.Records, nil)
	log.Info().Msgf("shutdown DNSProvider :%s", p.Describe())
	p.cancelFunc()
	return nil
}

func (p *DNSProvider) Config() *autodiscover.Config {
	return p.cfg
}

func (p *DNSProvider) Key() string {
	return p.Config().Key()
}

func (p *DNSProvider) GetCreator() creator.Creator {
	return p.Creator
}

func (p *DNSProvider) Describe() string {
	return "DNSProvider: " + p.Name
}
