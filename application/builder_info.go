package application

import (
	"context"
	"encoding/json"
	"time"

	"github.com/ethereum/go-ethereum/log"
)

type BuilderInfo struct {
	Name          string   `json:"name"`
	RPC           string   `json:"rpc"`
	SupportedApis []string `json:"supported-apis"`
}
type Fetcher interface {
	Fetch(ctx context.Context) ([]byte, error)
}
type BuilderInfoService struct {
	fetcher      Fetcher
	builderInfos []BuilderInfo
}

func StartBuilderInfoService(ctx context.Context, fetcher Fetcher, fetchInterval time.Duration) (*BuilderInfoService, error) {
	bis := BuilderInfoService{
		fetcher: fetcher,
	}
	if fetcher != nil {
		err := bis.fetchBuilderInfo(ctx)
		if err != nil {
			return nil, err
		}
		go bis.syncLoop(fetchInterval)

	}
	return &bis, nil
}
func (bis *BuilderInfoService) Builders() []BuilderInfo {
	return bis.builderInfos
}

func (bis *BuilderInfoService) BuilderNames() []string {
	var names = make([]string, 0, len(bis.builderInfos))
	for _, builderInfo := range bis.builderInfos {
		names = append(names, builderInfo.Name)
	}
	return names
}

func (bis *BuilderInfoService) syncLoop(fetchInterval time.Duration) {
	ticker := time.NewTicker(fetchInterval)
	for range ticker.C {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		err := bis.fetchBuilderInfo(ctx)
		if err != nil {
			//TODO: probably panic on multiple consequent errors, though it's not critical in nature
			log.Error("failed to fetch builder info", "err", err)
		}
		cancel()
	}
}

func (bis *BuilderInfoService) fetchBuilderInfo(ctx context.Context) error {
	bts, err := bis.fetcher.Fetch(ctx)
	if err != nil {
		return err
	}
	var builderInfos []BuilderInfo
	err = json.Unmarshal(bts, &builderInfos)
	if err != nil {
		return err
	}
	bis.builderInfos = builderInfos
	return nil
}
