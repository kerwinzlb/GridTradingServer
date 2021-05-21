package okex

/*
 OKEX api config info
 @author Lingting Fu
 @date 2018-12-27
 @version 1.0.0
*/

const (
	ACCOUNT_BALANCE = "/api/v5/account/balance"

	MARKET_TICKERS = "/api/v5/market/tickers"
	MARKET_TICKER  = "/api/v5/market/ticker"

	TRADE_ORDER               = "/api/v5/trade/order"
	TRADE_BATCH_ORDERS        = "/api/v5/trade/batch-orders"
	TRADE_CANCEL_ORDER        = "/api/v5/trade/cancel-order"
	TRADE_CANCLE_BATCH_ORDERS = "/api/v5/trade/cancel-batch-orders"
	TRADE_ORDERS_PENDING      = "/api/v5/trade/orders-pending"
)

const (
	OKEX_TIME_URI = "/api/general/v3/time"

	ACCOUNT_CURRENCIES                 = "/api/account/v3/currencies"
	ACCOUNT_DEPOSIT_ADDRESS            = "/api/account/v3/deposit/address"
	ACCOUNT_DEPOSIT_HISTORY            = "/api/account/v3/deposit/history"
	ACCOUNT_DEPOSIT_HISTORY_CURRENCY   = "/api/account/v3/deposit/history/{currency}"
	ACCOUNT_LEDGER                     = "/api/account/v3/ledger"
	ACCOUNT_WALLET                     = "/api/account/v3/wallet"
	ACCOUNT_WALLET_CURRENCY            = "/api/account/v3/wallet/{currency}"
	ACCOUNT_WITHRAWAL                  = "/api/account/v3/withdrawal"
	ACCOUNT_WITHRAWAL_FEE              = "/api/account/v3/withdrawal/fee"
	ACCOUNT_WITHRAWAL_HISTORY          = "/api/account/v3/withdrawal/history"
	ACCOUNT_WITHRAWAL_HISTORY_CURRENCY = "/api/account/v3/withdrawal/history/{currency}"
	ACCOUNT_TRANSFER                   = "/api/account/v3/transfer"

	FUTURES_RATE                          = "/api/futures/v3/rate"
	FUTURES_INSTRUMENTS                   = "/api/futures/v3/instruments"
	FUTURES_CURRENCIES                    = "/api/futures/v3/instruments/currencies"
	FUTURES_INSTRUMENT_BOOK               = "/api/futures/v3/instruments/{instrument_id}/book"
	FUTURES_TICKERS                       = "/api/futures/v3/instruments/ticker"
	FUTURES_INSTRUMENT_TICKER             = "/api/futures/v3/instruments/{instrument_id}/ticker"
	FUTURES_INSTRUMENT_TRADES             = "/api/futures/v3/instruments/{instrument_id}/trades"
	FUTURES_INSTRUMENT_CANDLES            = "/api/futures/v3/instruments/{instrument_id}/candles"
	FUTURES_INSTRUMENT_MARK_PRICE         = "/api/futures/v3/instruments/{instrument_id}/mark_price"
	FUTURES_INSTRUMENT_INDEX              = "/api/futures/v3/instruments/{instrument_id}/index"
	FUTURES_INSTRUMENT_ESTIMATED_PRICE    = "/api/futures/v3/instruments/{instrument_id}/estimated_price"
	FUTURES_INSTRUMENT_OPEN_INTEREST      = "/api/futures/v3/instruments/{instrument_id}/open_interest"
	FUTURES_INSTRUMENT_PRICE_LIMIT        = "/api/futures/v3/instruments/{instrument_id}/price_limit"
	FUTURES_INSTRUMENT_LIQUIDATION        = "/api/futures/v3/instruments/{instrument_id}/liquidation"
	FUTURES_POSITION                      = "/api/futures/v3/position"
	FUTURES_INSTRUMENT_POSITION           = "/api/futures/v3/{instrument_id}/position"
	FUTURES_ACCOUNTS                      = "/api/futures/v3/accounts"
	FUTURES_ACCOUNTS_LIQUI_MODE           = "/api/futures/v3/accounts/liqui_mode"
	FUTURES_ACCOUNTS_MARGIN_MODE          = "/api/futures/v3/accounts/margin_mode"
	FUTURES_ACCOUNT_CURRENCY_INFO         = "/api/futures/v3/accounts/{currency}"
	FUTURES_ACCOUNT_CURRENCY_LEDGER       = "/api/futures/v3/accounts/{currency}/ledger"
	FUTURES_ACCOUNT_CURRENCY_LEVERAGE     = "/api/futures/v3/accounts/{currency}/leverage"
	FUTURES_ACCOUNT_INSTRUMENT_HOLDS      = "/api/futures/v3/accounts/{instrument_id}/holds"
	FUTURES_ORDER                         = "/api/futures/v3/order"
	FUTURES_ORDERS                        = "/api/futures/v3/orders"
	FUTURES_INSTRUMENT_ORDER_LIST         = "/api/futures/v3/orders/{instrument_id}"
	FUTURES_INSTRUMENT_ORDER_INFO         = "/api/futures/v3/orders/{instrument_id}/{order_client_id}"
	FUTURES_INSTRUMENT_ORDER_CANCEL       = "/api/futures/v3/cancel_order/{instrument_id}/{order_client_id}"
	FUTURES_INSTRUMENT_ORDER_BATCH_CANCEL = "/api/futures/v3/cancel_batch_orders/{instrument_id}"
	FUTURES_FILLS                         = "/api/futures/v3/fills"

	MARGIN_ACCOUNTS                         = "/api/margin/v3/accounts"
	MARGIN_ACCOUNTS_INSTRUMENT              = "/api/margin/v3/accounts/{instrument_id}"
	MARGIN_ACCOUNTS_INSTRUMENT_LEDGER       = "/api/margin/v3/accounts/{instrument_id}/ledger"
	MARGIN_ACCOUNTS_AVAILABILITY            = "/api/margin/v3/accounts/availability"
	MARGIN_ACCOUNTS_INSTRUMENT_AVAILABILITY = "/api/margin/v3/accounts/{instrument_id}/availability"
	MARGIN_ACCOUNTS_BORROWED                = "/api/margin/v3/accounts/borrowed"
	MARGIN_ACCOUNTS_INSTRUMENT_BORROWED     = "/api/margin/v3/accounts/{instrument_id}/borrowed"
	MARGIN_ACCOUNTS_BORROW                  = "/api/margin/v3/accounts/borrow"
	MARGIN_ACCOUNTS_REPAYMENT               = "/api/margin/v3/accounts/repayment"
	MARGIN_ORDERS                           = "/api/margin/v3/orders"
	MARGIN_BATCH_ORDERS                     = "/api/margin/v3/batch_orders"
	MARGIN_CANCEL_ORDERS_BY_ID              = "/api/margin/v3/cancel_orders/{order_client_id}"
	MARGIN_CANCEL_BATCH_ORDERS              = "/api/margin/v3/cancel_batch_orders"
	MARGIN_ORDERS_BY_ID                     = "/api/margin/v3/orders/{order_client_id}"
	MARGIN_ORDERS_PENDING                   = "/api/margin/v3/orders_pending"
	MARGIN_FILLS                            = "/api/margin/v3/fills"

	SPOT_ACCOUNTS                 = "/api/spot/v3/accounts"
	SPOT_ACCOUNTS_CURRENCY        = "/api/spot/v3/accounts/{currency}"
	SPOT_ACCOUNTS_CURRENCY_LEDGER = "/api/spot/v3/accounts/{currency}/ledger"
	SPOT_ORDERS                   = "/api/spot/v3/orders"
	SPOT_BATCH_ORDERS             = "/api/spot/v3/batch_orders"
	SPOT_CANCEL_ORDERS_BY_ID      = "/api/spot/v3/cancel_orders/{order_client_id}"
	SPOT_CANCEL_BATCH_ORDERS      = "/api/spot/v3/cancel_batch_orders"
	SPOT_ORDERS_PENDING           = "/api/spot/v3/orders_pending"
	SPOT_ORDERS_BY_ID             = "/api/spot/v3/orders/{order_client_id}"
	SPOT_FILLS                    = "/api/spot/v3/fills"
	SPOT_INSTRUMENTS              = "/api/spot/v3/instruments"
	SPOT_INSTRUMENT_BOOK          = "/api/spot/v3/instruments/{instrument_id}/book"
	SPOT_INSTRUMENTS_TICKER       = "/api/spot/v3/instruments/ticker"
	SPOT_INSTRUMENT_TICKER        = "/api/spot/v3/instruments/{instrument_id}/ticker"
	SPOT_INSTRUMENT_TRADES        = "/api/spot/v3/instruments/{instrument_id}/trades"
	SPOT_INSTRUMENT_CANDLES       = "/api/spot/v3/instruments/{instrument_id}/candles"

	SWAP_INSTRUMENT_ACCOUNT                 = "/api/swap/v3/{instrument_id}/accounts"
	SWAP_INSTRUMENT_POSITION                = "/api/swap/v3/{instrument_id}/position"
	SWAP_ACCOUNTS                           = "/api/swap/v3/accounts"
	SWAP_ACCOUNTS_HOLDS                     = "/api/swap/v3/accounts/{instrument_id}/holds"
	SWAP_ACCOUNTS_LEDGER                    = "/api/swap/v3/accounts/{instrument_id}/ledger"
	SWAP_ACCOUNTS_LEVERAGE                  = "/api/swap/v3/accounts/{instrument_id}/leverage"
	SWAP_ACCOUNTS_SETTINGS                  = "/api/swap/v3/accounts/{instrument_id}/settings"
	SWAP_FILLS                              = "/api/swap/v3/fills"
	SWAP_INSTRUMENTS                        = "/api/swap/v3/instruments"
	SWAP_INSTRUMENTS_TICKER                 = "/api/swap/v3/instruments/ticker"
	SWAP_INSTRUMENT_CANDLES                 = "/api/swap/v3/instruments/{instrument_id}/candles"
	SWAP_INSTRUMENT_DEPTH                   = "/api/swap/v3/instruments/{instrument_id}/depth"
	SWAP_INSTRUMENT_FUNDING_TIME            = "/api/swap/v3/instruments/{instrument_id}/funding_time"
	SWAP_INSTRUMENT_HISTORICAL_FUNDING_RATE = "/api/swap/v3/instruments/{instrument_id}/historical_funding_rate"
	SWAP_INSTRUMENT_INDEX                   = "/api/swap/v3/instruments/{instrument_id}/index"
	SWAP_INSTRUMENT_LIQUIDATION             = "/api/swap/v3/instruments/{instrument_id}/liquidation"
	SWAP_INSTRUMENT_MARK_PRICE              = "/api/swap/v3/instruments/{instrument_id}/mark_price"
	SWAP_INSTRUMENT_OPEN_INTEREST           = "/api/swap/v3/instruments/{instrument_id}/open_interest"
	SWAP_INSTRUMENT_PRICE_LIMIT             = "/api/swap/v3/instruments/{instrument_id}/price_limit"
	SWAP_INSTRUMENT_TICKER                  = "/api/swap/v3/instruments/{instrument_id}/ticker"
	SWAP_INSTRUMENT_TRADES                  = "/api/swap/v3/instruments/{instrument_id}/trades"
	SWAP_INSTRUMENT_ORDER_LIST              = "/api/swap/v3/orders/{instrument_id}"
	SWAP_INSTRUMENT_ORDER_BY_ID             = "/api/swap/v3/orders/{instrument_id}/{order_client_id}"
	SWAP_RATE                               = "/api/swap/v3/rate"
	SWAP_ORDER                              = "/api/swap/v3/order"
	SWAP_ORDERS                             = "/api/swap/v3/orders"
	SWAP_POSITION                           = "/api/swap/v3/position"

	SWAP_CANCEL_BATCH_ORDERS = "/api/swap/v3/cancel_batch_orders/{instrument_id}"
	SWAP_CANCEL_ORDER        = "/api/swap/v3/cancel_order/{instrument_id}/{order_id}"
)
