package stats

import(
    "github.com/go-redis/redis"
    "go.uber.org/zap"
    "encoding/json"
)

type Stats struct {
    StatsCount uint8
    BlockTime uint64
    Difficulty uint64
    UncleRate uint8
    LastBlockTime uint32
    Window uint8
    RedisPrefix string
    HistoryWindow uint16
    Redis *redis.Client
    Log *zap.SugaredLogger
    KnownMiners map[string]string
    Miners map[string]int
}

type Miner struct {
    Name string     `json:"name"`
    Count uint8     `json:"count"`
    Percent float32 `json:"percent"`
}

func (s *Stats) Populate(blockNum int64, timeStamp uint32, difficulty int64, uncles uint8, miner string) (res bool) {
    res = false

    if len(s.Miners) == 0 {
        s.Miners = make(map[string]int)
    }
    if len(s.KnownMiners) == 0 {
        s.KnownMiners, _ = s.Redis.HGetAll(s.RedisPrefix+"known_miners").Result()
    }

	var blockTime uint32 = timeStamp - s.LastBlockTime
    s.LastBlockTime = timeStamp

    s.BlockTime = s.BlockTime + uint64(blockTime)
    s.Difficulty += uint64(difficulty)
    s.UncleRate += uncles
    minerName, _ := s.KnownMiners["_miner_"+miner]
    if(minerName == "") {
        minerName = "Unknown"
    }
    s.Miners[minerName]++
    s.StatsCount++

	if(s.StatsCount == s.Window) {
        s.Log.Debugf("Calculating stats after window: %d", s.Window)
        var currentHashRate float64 = float64(s.Difficulty)/float64(s.BlockTime)/1000/1000/1000
        var currentDifficulty float64 = float64(s.Difficulty)/float64(s.StatsCount)/1000/1000/1000
        var currentBlockTime float32 = float32(s.BlockTime)/float32(s.StatsCount)
        var currentUncleRate float32 = (float32(s.UncleRate)/float32(s.StatsCount)) * 100

        s.Redis.Set(s.RedisPrefix+"current_unclerate", currentUncleRate, 0)
        s.Redis.Set(s.RedisPrefix+"current_hashrate", currentHashRate, 0)
        s.Redis.Set(s.RedisPrefix+"current_difficulty", currentDifficulty, 0)
        s.Redis.Set(s.RedisPrefix+"current_blocktime", currentBlockTime, 0)

        var historySize int64 = 0

        historySize, _ = s.Redis.LPush(s.RedisPrefix+"history_unclerate", currentUncleRate).Result()
        if(historySize > int64(s.HistoryWindow)) {
            s.Redis.RPop(s.RedisPrefix+"history_unclerate")
        }
        historySize, _ = s.Redis.LPush(s.RedisPrefix+"history_hashrate", currentHashRate).Result()
        if(historySize > int64(s.HistoryWindow)) {
            s.Redis.RPop(s.RedisPrefix+"history_hashrate")
        }
        historySize, _ = s.Redis.LPush(s.RedisPrefix+"history_difficulty", currentDifficulty).Result()
        if(historySize > int64(s.HistoryWindow)) {
            s.Redis.RPop(s.RedisPrefix+"history_difficulty")
        }
        historySize, _ = s.Redis.LPush(s.RedisPrefix+"history_blocktime", currentBlockTime).Result()
        if(historySize > int64(s.HistoryWindow)) {
            s.Redis.RPop(s.RedisPrefix+"history_blocktime")
        }

        s.Redis.Del(s.RedisPrefix+"top_miners")

        sortedMiners := sortMapByValue(s.Miners)
        for m := range sortedMiners {
            name := sortedMiners[m].Key
            blocks := sortedMiners[m].Value
            minerData := &Miner{Name: name, Count: uint8(blocks), Percent: float32((float32(blocks)/float32(s.StatsCount))*100)}
            minerDataJSON, _ := json.Marshal(minerData)
            s.Redis.LPush(s.RedisPrefix+"top_miners", minerDataJSON)
        }

        s.Miners = make(map[string]int)
        s.StatsCount = 0
        s.BlockTime = 0
        s.Difficulty = 0
        s.UncleRate = 0
	}
    return res
}
