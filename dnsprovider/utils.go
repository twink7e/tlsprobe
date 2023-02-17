package dnsprovider

import (
	"context"
	"github.com/rs/zerolog/log"
	"tlsprobe/common"
	"tlsprobe/common/creator"
)

func RecordToHostScannerConfig(record *Record) []common.HostScannerConfig {
	cfgs := make([]common.HostScannerConfig, len(record.Value))
	for i, v := range record.Value {
		cfg := common.HostScannerConfig{
			Host: v,
			TLSOptions: common.TLSCheckOptions{
				Domain:             GetFQDN(record),
				Timeout:            10000,
				InsecureSkipVerify: true,
				ReTryTimes:         3,
			},
		}
		cfgs[i] = cfg
	}
	return cfgs
}

func MakeHostScanner(ctx context.Context, record *Record, creator creator.Creator) {
	for _, v := range record.Value {
		cfg := common.HostScannerConfig{
			Host: v,
			TLSOptions: common.TLSCheckOptions{
				Domain:             GetFQDN(record),
				Timeout:            10000,
				InsecureSkipVerify: true,
				ReTryTimes:         3,
			},
		}
		common.Exp.UpdateHostScannerConfig(ctx, &cfg, creator)
	}
}

func GetRealRecords(record *Record) []*Record {
	records := make([]*Record, 0)
	if record == nil {
		return nil
	}
	if record.Value != nil {
		records = append(records, record)
	}
	for _, r := range record.Children {
		if len(r.Children) > 0 {
			records = append(records, GetRealRecords(r)...)
		}
		if len(r.Value) > 0 {
			records = append(records, r)
		}
	}
	return records
}

func RefreshResources(oldRecords, newRecords map[string]*Record) {
	records := GetShouldRefreshRecords(oldRecords, newRecords)
	for _, record := range records {
		for _, cfg := range RecordToHostScannerConfig(record) {
			log.Info().Msgf("GetShouldRefreshRecords should delete hostScanner: %s", cfg.Key())
			common.Exp.RemoveHostScanner(cfg.Key())
		}
	}
}

func GetShouldRefreshRecords(old, newRecords map[string]*Record) (shouldDeleteRecords []*Record) {
	if old == nil {
		return
	}
	shouldDeleteRecords = make([]*Record, 0)
	for _, record := range old {
		if newRecords == nil {
			shouldDeleteRecords = append(shouldDeleteRecords, GetRealRecords(record)...)
			continue
		}
		r, exists := newRecords[record.Record]
		if !exists || r == nil || r.Children == nil {
			shouldDeleteRecords = append(shouldDeleteRecords, GetRealRecords(record)...)
			continue
		}
		if record.Children != nil {
			shouldDeleteRecords = append(shouldDeleteRecords, GetShouldRefreshRecords(record.Children, r.Children)...)
		}
	}
	return
}
