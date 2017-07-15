package main

import (
    "strconv"
    "time"
    "fmt"
	. "github.com/jeffdoubleyou/EthereumAPI"
    "github.com/go-redis/redis"
    "github.com/spf13/viper"
    "go.uber.org/zap"
    "./src"
)

type Stats struct {
    StatsCount uint8
    BlockTime uint8
    Difficulty uint32
    HashRate uint32
    UncleRate uint8
    LastBlock uint32
    ResetAt uint8
}

var log *zap.SugaredLogger
var RedisClient *redis.Client

func main() {

    viper.SetConfigName("config")
    viper.AddConfigPath(".")
    viper.SetDefault("redis.host", "127.0.0.1")
    viper.SetDefault("redis.port", "6379")
    viper.SetDefault("redis.password", "")
    viper.SetDefault("node.host", "127.0.0.1")
    viper.SetDefault("node.port", "8545")
    viper.SetDefault("limits.recentBlocks", 10)
    viper.SetDefault("limits.recentTransactions", 25)
    viper.SetDefault("log.path", "./blockwatcher.log")
    viper.SetDefault("log.environment", "production")

    err := viper.ReadInConfig();
    if(err != nil) {
        panic(fmt.Errorf("Unable to open config : %s\n", err))
    }

    var logger *zap.Logger

    if(viper.GetString("log.environment") == "development") {
        logger, _ = zap.NewDevelopment()
    } else {
        logger, _ = zap.NewProduction()
    }

    defer logger.Sync()
    log = logger.Sugar()
    log.Info("Starting up")

    redisClient := redis.NewClient(&redis.Options{
        Addr:   viper.GetString("redis.host")+":"+viper.GetString("redis.port"),
        Password: viper.GetString("redis.password"),
        DB: 0,
    })

    SetServer(viper.GetString("node.host")+":"+viper.GetString("node.port"))

    currentBlock := getCurrentBlockNumber()
    var lastBlock int64 = 0


    run := 1

    Stats := &stats.Stats{Window: uint8(viper.GetInt64("limits.window")),Redis:redisClient,Log:log, RedisPrefix: viper.GetString("redis.prefix"), HistoryWindow: 2000}

    for run == 1 {
        lastBlock = currentBlock - viper.GetInt64("limits.window") + 1;
        log.Debugf("Starting at block %d up to block %d", lastBlock, currentBlock)
        initialBlock, _ := EthGetBlockByNumber(strconv.FormatInt(lastBlock-1,10),true)
        initialBlockTime, _ := ParseQuantity(initialBlock.Timestamp)
        Stats.LastBlockTime = uint32(initialBlockTime)
        for lastBlock <= currentBlock {
            log.Debugf("Going to get block #%d", lastBlock)
            block, err  := EthGetBlockByNumber(strconv.FormatInt(lastBlock, 10), true)

            if err != nil {
                log.Errorf("%s\n", err)
                run = 0
            }

            timeStamp, _ := ParseQuantity(block.Timestamp)
            difficulty, _ := ParseQuantity(block.Difficulty)
            Stats.Populate(lastBlock,uint32(timeStamp),difficulty,uint8(len(block.Uncles)),block.Miner)
            lastBlock++
        }
        log.Infof("All cought up, waiting for a new block");
        for currentBlock == lastBlock {
            time.Sleep(5000*time.Millisecond)
            currentBlock = getCurrentBlockNumber()
            log.Infof("Current Block: %d Last Block: %d", currentBlock, lastBlock)
        }
    }
}

func getCurrentBlockNumber()(n int64) {
    n, err := EthBlockNumber()
    if(err != nil) {
        log.Errorf("Error getting current block number: %s", err)
        n = 0;
    }
    return
}

