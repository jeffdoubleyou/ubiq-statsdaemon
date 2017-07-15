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
    Miners map[string]uint8
}

type Miner struct {
    Name string     `json:"name"`
    Count uint8     `json:"count"`
    Percent float32 `json:"percent"`
}

func (s *Stats) Populate(blockNum int64, timeStamp uint32, difficulty int64, uncles uint8, miner string) (res bool) {
    res = false

    if len(s.Miners) == 0 {
        s.Miners = make(map[string]uint8)
    }
    if len(s.KnownMiners) == 0 {
        s.KnownMiners, _ = s.Redis.HGetAll("explorer::known_miners").Result()
    }

	var blockTime uint32 = timeStamp - s.LastBlockTime
    s.LastBlockTime = timeStamp
	if(s.StatsCount == s.Window) {
        s.Log.Debugf("Calculating stats after window: %d", s.Window)
        var currentHashRate uint64 = s.Difficulty/s.BlockTime
        var currentDifficulty uint64 = s.Difficulty / uint64(s.StatsCount)
        var currentBlockTime float32 = float32(s.BlockTime) / float32(s.StatsCount)
        var currentUncleRate float32 = float32(s.UncleRate) / float32(s.StatsCount)

        s.Redis.Set(s.RedisPrefix+"::current_unclerate", currentUncleRate, 0)
        s.Redis.Set(s.RedisPrefix+"::current_hashrate", currentHashRate, 0)
        s.Redis.Set(s.RedisPrefix+"::current_difficulty", currentDifficulty, 0)
        s.Redis.Set(s.RedisPrefix+"::current_blocktime", currentBlockTime, 0)

        var historySize int64 = 0

        historySize, _ = s.Redis.LPush(s.RedisPrefix+"::history_unclerate", currentUncleRate).Result()
        if(historySize > int64(s.HistoryWindow)) {
            s.Redis.RPop(s.RedisPrefix+"::history_unclerate")
        }
        historySize, _ = s.Redis.LPush(s.RedisPrefix+"::history_hashrate", currentHashRate).Result()
        if(historySize > int64(s.HistoryWindow)) {
            s.Redis.RPop(s.RedisPrefix+"::history_hashrate")
        }
        historySize, _ = s.Redis.LPush(s.RedisPrefix+"::history_difficulty", currentDifficulty).Result()
        if(historySize > int64(s.HistoryWindow)) {
            s.Redis.RPop(s.RedisPrefix+"::history_difficulty")
        }
        historySize, _ = s.Redis.LPush(s.RedisPrefix+"::history_blocktime", currentBlockTime).Result()
        if(historySize > int64(s.HistoryWindow)) {
            s.Redis.RPop(s.RedisPrefix+"::history_blocktime")
        }

        s.Redis.Del(s.RedisPrefix+"::top_miners")

        for name, blocks := range s.Miners {
            minerData := &Miner{Name: name, Count: uint8(blocks), Percent: float32(blocks/s.StatsCount*100)}
            minerDataJSON, _ := json.Marshal(minerData)
            s.Redis.LPush(s.RedisPrefix+"::top_miners", minerDataJSON)
        }

        s.Miners = make(map[string]uint8)
        s.StatsCount = 0
        s.BlockTime = 0
        s.Difficulty = 0
        s.UncleRate = 0
	}
    s.BlockTime = s.BlockTime + uint64(blockTime)
    s.Difficulty += uint64(difficulty)
    s.UncleRate += uncles
    minerName, _ := s.KnownMiners["_miner_"+miner]
    if(minerName == "") {
        minerName = "Unknown"
    }
    s.Miners[minerName]++
    s.StatsCount++

    return res
}

/*
my $pct = $miners{$miner} / $stats_reset * 100;
            $redis->lpush('explorer::top_miners', $json->encode({ name => $miner, count => $miners{$miner}, percent => $pct}));

        $redis->set('explorer::current_unclerate' => $current_uncle_rate);
        $redis->set('explorer::current_difficulty' => $current_difficulty);
        $redis->set('explorer::current_blocktime' => $current_block_time);
        $redis->set('explorer::current_hashrate' => $current_hash_rate);

        $redis->lpush('explorer::history_difficulty",g $current_difficulty);
        $redis->lpush('explorer::history_blocktime",g $current_block_time);
        $redis->lpush('explorer::history_hashrate",g $current_hash_rate);

    my $block_time = $block->{'timestamp'}-$last_block_time;
    $last_block_time = $block->{'timestamp'};

STUFF

    $block_time_sum += $block_time;
    $difficulty_sum += $block->{'difficulty'};
    my $miner = $known_miners{'_miner_'.$block->{'miner'}} || 'Unknown';
    $miners{$miner}||=0;
    $miners{$miner}++;
    $uncle_count += scalar(@{$block->{'uncles'}});
    $stats_count++;
*/
