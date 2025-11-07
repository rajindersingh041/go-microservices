package database

const LogSql = `
CREATE TABLE IF NOT EXISTS mydatabase.events (
    Timestamp DateTime,
    Level     String,
    Source    String,
    Message   String
) ENGINE = MergeTree()
ORDER BY Timestamp;
`

const upstoxMarketDataQuote = `
CREATE TABLE IF NOT EXISTS mydatabase.market_data (
    request_id     String,
    response_time  DateTime64(3),
    token_name     String,
    timestamp     DateTime,
    last_trade_time DateTime,
    last_price     Float64,
    close_price    Float64,
    net_change     Float64,
    ohlc_open      Float64,
    ohlc_high      Float64,
    ohlc_low       Float64,
    ohlc_close     Float64,
    ohlc_volume    Int64
) ENGINE = MergeTree()
ORDER BY (token_name, timestamp);
`

// "timestamp": "2025-11-07 06:52:38",
// "lastTradeTime": "2025-11-07 04:00:00",
// "lastPrice": 25492.3,
// "closePrice": 25509.7,
// "lastQuantity": 0,
// "netChange": -17.4,
// "yl": 21743.65,
// "yh": 26104.2,
// "ohlc": {
// 	"interval": "1d",
// 	"open": 25433.8,
// 	"high": 25551.25,
// 	"low": 25318.45,
// 	"close": 25509.7,
// 	"volume": 0,
// 	"ts": 1762453800000
// }