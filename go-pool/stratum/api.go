package stratum

import (
	"encoding/json"
	"net/http"
	"sync/atomic"
	"time"

	"../util"
)

func (s *StratumServer) StatsIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)

	hashrate, hashrate24h, totalOnline, miners := s.collectMinersStats()
	stats := map[string]interface{}{
		"miners":      miners,
		"hashrate":    hashrate,
		"hashrate24h": hashrate24h,
		"totalMiners": len(miners),
		"totalOnline": totalOnline,
		"timedOut":    len(miners) - totalOnline,
		"now":         util.MakeTimestamp(),
	}

	stats["luck"] = s.getLuckStats()

	if t := s.currentBlockTemplate(); t != nil {
		stats["height"] = t.Height
		stats["diff"] = t.Difficulty
		roundShares := atomic.LoadInt64(&s.roundShares)
		stats["variance"] = float64(roundShares) / float64(t.Difficulty)
		stats["template"] = true
	}
	json.NewEncoder(w).Encode(stats)
}

func (s *StratumServer) collectMinersStats() (float64, float64, int, []interface{}) {
	now := util.MakeTimestamp()
	var result []interface{}
	totalhashrate := float64(0)
	totalhashrate24h := float64(0)
	totalOnline := 0
	window24h := 24 * time.Hour

	for m := range s.miners.Iter() {
		stats := make(map[string]interface{})
		lastBeat := m.Val.getLastBeat()
		hashrate := m.Val.hashrate(s.estimationWindow)
		hashrate24h := m.Val.hashrate(window24h)
		totalhashrate += hashrate
		totalhashrate24h += hashrate24h
		stats["name"] = m.Key
		stats["hashrate"] = hashrate
		stats["hashrate24h"] = hashrate24h
		stats["lastBeat"] = lastBeat
		stats["validShares"] = atomic.LoadUint64(&m.Val.validShares)
		stats["invalidShares"] = atomic.LoadUint64(&m.Val.invalidShares)
		stats["accepts"] = atomic.LoadUint64(&m.Val.accepts)
		stats["rejects"] = atomic.LoadUint64(&m.Val.rejects)
		if !s.config.Frontend.HideIP {
			stats["ip"] = m.Val.IP
		}

		if now-lastBeat > (int64(s.timeout/2) / 1000000) {
			stats["warning"] = true
		}
		if now-lastBeat > (int64(s.timeout) / 1000000) {
			stats["timeout"] = true
		} else {
			totalOnline++
		}
		result = append(result, stats)
	}
	return totalhashrate, totalhashrate24h, totalOnline, result
}

func (s *StratumServer) getLuckStats() map[string]interface{} {
	now := util.MakeTimestamp()
	var variance float64
	var totalVariance float64
	var blocksCount int
	var totalBlocksCount int

	s.blocksMu.Lock()
	defer s.blocksMu.Unlock()

	for k, v := range s.blockStats {
		if k >= now-int64(s.luckWindow) {
			blocksCount++
			variance += v
		}
		if k >= now-int64(s.luckLargeWindow) {
			totalBlocksCount++
			totalVariance += v
		} else {
			delete(s.blockStats, k)
		}
	}
	if blocksCount != 0 {
		variance = variance / float64(blocksCount)
	}
	if totalBlocksCount != 0 {
		totalVariance = totalVariance / float64(totalBlocksCount)
	}
	result := make(map[string]interface{})
	result["variance"] = variance
	result["blocksCount"] = blocksCount
	result["window"] = s.config.LuckWindow
	result["totalVariance"] = totalVariance
	result["totalBlocksCount"] = totalBlocksCount
	result["largeWindow"] = s.config.LargeLuckWindow
	return result
}
