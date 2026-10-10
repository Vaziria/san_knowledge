//+------------------------------------------------------------------+
//|                                              AutotradeBridge.mq5 |
//| The MetaTrader 5 side of autotrade's second venue                |
//| (golang/apps/autotrade, USAGE.md "MetaTrader 5").                 |
//|                                                                  |
//| autotrade writes each request as a file autotrade\<id>.req in    |
//| the terminals' common Files folder                               |
//| (%APPDATA%\MetaQuotes\Terminal\Common\Files). This EA picks it   |
//| up on a timer, answers it in autotrade\<id>.res, and trades on a |
//| demo account only.                                               |
//|                                                                  |
//| A request is key=value lines; op= says what to do. An answer is  |
//| "ok" or "error<TAB>code<TAB>message" on its first line, then one |
//| record per line: key=value fields separated by tabs. Times are   |
//| UTC Unix seconds, volumes are lots.                              |
//+------------------------------------------------------------------+
#property copyright   "autotrade"
#property version     "1.00"
#property description "Answers autotrade's requests in Common\\Files\\autotrade. Trades on a demo account only."

input int  PollMillis      = 20;       // How often to look for requests (ms)
input long Magic           = 20261010; // Magic number of autotrade's orders
input int  DeviationPoints = 50;       // Slippage allowed on market orders (points)

#define BRIDGE_DIR     "autotrade\\"
#define BRIDGE_VERSION "1"

string g_keys[];   // the request being answered
string g_vals[];
string g_out = ""; // the answer's records so far
string g_rec = ""; // the record being built

//+------------------------------------------------------------------+
int OnInit()
  {
   FolderCreate("autotrade", FILE_COMMON);
   if(!EventSetMillisecondTimer(PollMillis))
      return INIT_FAILED;
   PrintFormat("autotrade bridge %s: answering requests in %s\\Files\\autotrade for account %I64d on %s (%s)",
               BRIDGE_VERSION, TerminalInfoString(TERMINAL_COMMONDATA_PATH), AccountInfoInteger(ACCOUNT_LOGIN),
               AccountInfoString(ACCOUNT_SERVER), IsDemo() ? "demo" : "NOT A DEMO ACCOUNT: trading is refused");
   return INIT_SUCCEEDED;
  }

void OnDeinit(const int reason)
  {
   EventKillTimer();
  }

void OnTimer()
  {
   string name;
   string names[];
   long h = FileFindFirst(BRIDGE_DIR + "*.req", name, FILE_COMMON);
   if(h == INVALID_HANDLE)
      return;
   do
     {
      int n = ArraySize(names);
      ArrayResize(names, n + 1);
      names[n] = name;
     }
   while(FileFindNext(h, name));
   FileFindClose(h);
   for(int i = 0; i < ArraySize(names); i++)
      Handle(names[i]);
  }

//+------------------------------------------------------------------+
//| Handle answers one request file.                                 |
//+------------------------------------------------------------------+
void Handle(const string name)
  {
   string id = StringSubstr(name, 0, StringLen(name) - 4); // without ".req"
   string work = BRIDGE_DIR + id + ".work";
   // Claim it first: autotrade deletes a request nobody claimed when it
   // gives up waiting, and only one of the two can succeed.
   if(!FileMove(BRIDGE_DIR + name, FILE_COMMON, work, FILE_COMMON | FILE_REWRITE))
      return;
   string text = ReadAll(work);
   FileDelete(work, FILE_COMMON);

   Parse(text);
   g_out = "";
   g_rec = "";
   string answer = Answer();

   string tmp = BRIDGE_DIR + id + ".tmp";
   int f = FileOpen(tmp, FILE_WRITE | FILE_TXT | FILE_ANSI | FILE_COMMON);
   if(f == INVALID_HANDLE)
     {
      PrintFormat("autotrade bridge: cannot write %s (error %d)", tmp, GetLastError());
      return;
     }
   FileWriteString(f, answer);
   FileClose(f);
   if(!FileMove(tmp, FILE_COMMON, BRIDGE_DIR + id + ".res", FILE_COMMON | FILE_REWRITE))
      PrintFormat("autotrade bridge: cannot answer %s (error %d)", id, GetLastError());
  }

string Answer()
  {
   string op = Arg("op");
   if(op == "ping")
      return OpPing();
   if(op == "symbol")
      return OpSymbol();
   if(op == "rates")
      return OpRates();
   if(op == "positions")
      return OpPositions();
   if(op == "orders")
      return OpOrders();
   if(op == "order")
      return OpOrder();
   if(op == "deals")
      return OpDeals();
   if(op == "deal")
      return OpDeal();
   if(op == "close")
      return OpClose();
   if(op == "limit")
      return OpLimit();
   if(op == "sltp")
      return OpSLTP();
   if(op == "modify")
      return OpModify();
   if(op == "cancel")
      return OpCancel();
   return Err("op", "unknown op " + op);
  }

//+------------------------------------------------------------------+
//| Requests and answers                                             |
//+------------------------------------------------------------------+
string ReadAll(const string path)
  {
   int f = FileOpen(path, FILE_READ | FILE_TXT | FILE_ANSI | FILE_COMMON);
   if(f == INVALID_HANDLE)
      return "";
   string s = "";
   while(!FileIsEnding(f))
      s += FileReadString(f) + "\n";
   FileClose(f);
   return s;
  }

void Parse(const string text)
  {
   ArrayResize(g_keys, 0);
   ArrayResize(g_vals, 0);
   string lines[];
   int n = StringSplit(text, '\n', lines);
   for(int i = 0; i < n; i++)
     {
      string l = lines[i];
      StringTrimLeft(l);
      StringTrimRight(l);
      int eq = StringFind(l, "=");
      if(eq <= 0)
         continue;
      int k = ArraySize(g_keys);
      ArrayResize(g_keys, k + 1);
      ArrayResize(g_vals, k + 1);
      g_keys[k] = StringSubstr(l, 0, eq);
      g_vals[k] = StringSubstr(l, eq + 1);
     }
  }

string Arg(const string key)
  {
   for(int i = 0; i < ArraySize(g_keys); i++)
      if(g_keys[i] == key)
         return g_vals[i];
   return "";
  }

double ArgD(const string key) { return StringToDouble(Arg(key)); }
long   ArgL(const string key) { return StringToInteger(Arg(key)); }

// Clean keeps a value on its line and in its field.
string Clean(string s)
  {
   StringReplace(s, "\t", " ");
   StringReplace(s, "\r", " ");
   StringReplace(s, "\n", " ");
   return s;
  }

void Field(const string key, const string value)
  {
   if(g_rec != "")
      g_rec += "\t";
   g_rec += key + "=" + Clean(value);
  }

void FieldD(const string key, const double v) { Field(key, DoubleToString(v, 8)); }
void FieldL(const string key, const long v)   { Field(key, IntegerToString(v)); }

void EndRecord()
  {
   g_out += g_rec + "\n";
   g_rec = "";
  }

string Ok()
  {
   return "ok\n" + g_out;
  }

string Err(const string code, const string msg)
  {
   g_out = "";
   g_rec = "";
   return "error\t" + Clean(code) + "\t" + Clean(msg) + "\n";
  }

//+------------------------------------------------------------------+
//| Time: the trade server's clock runs in the broker's time zone.   |
//+------------------------------------------------------------------+
long GmtOffset()
  {
   long d = (long)(TimeTradeServer() - TimeGMT());
   return (long)MathRound(d / 900.0) * 900;
  }

datetime ServerTime(const long utc) { return (datetime)(utc + GmtOffset()); }

//+------------------------------------------------------------------+
//| Account                                                          |
//+------------------------------------------------------------------+
bool IsDemo()
  {
   return AccountInfoInteger(ACCOUNT_TRADE_MODE) == ACCOUNT_TRADE_MODE_DEMO;
  }

// TradeRefusal says why an order can't be sent now, or "".
string TradeRefusal()
  {
   if(!IsDemo())
      return "account " + IntegerToString(AccountInfoInteger(ACCOUNT_LOGIN)) + " is not a demo account: autotrade trades on demo accounts only";
   if(!TerminalInfoInteger(TERMINAL_CONNECTED))
      return "the terminal is not connected to the trade server";
   if(!TerminalInfoInteger(TERMINAL_TRADE_ALLOWED))
      return "Algo Trading is off in the terminal: press Algo Trading on the toolbar";
   if(!MQLInfoInteger(MQL_TRADE_ALLOWED))
      return "the EA may not trade: tick Allow Algo Trading in its settings (Common tab)";
   if(!AccountInfoInteger(ACCOUNT_TRADE_EXPERT))
      return "the trade server does not allow Expert Advisors to trade on this account";
   return "";
  }

string TradeModeName()
  {
   long m = AccountInfoInteger(ACCOUNT_TRADE_MODE);
   if(m == ACCOUNT_TRADE_MODE_DEMO)
      return "demo";
   if(m == ACCOUNT_TRADE_MODE_CONTEST)
      return "contest";
   return "real";
  }

string MarginModeName()
  {
   long m = AccountInfoInteger(ACCOUNT_MARGIN_MODE);
   if(m == ACCOUNT_MARGIN_MODE_RETAIL_HEDGING)
      return "hedging";
   if(m == ACCOUNT_MARGIN_MODE_EXCHANGE)
      return "exchange";
   return "netting";
  }

bool Hedging()
  {
   return AccountInfoInteger(ACCOUNT_MARGIN_MODE) == ACCOUNT_MARGIN_MODE_RETAIL_HEDGING;
  }

string OpPing()
  {
   Field("bridge", BRIDGE_VERSION);
   FieldL("login", AccountInfoInteger(ACCOUNT_LOGIN));
   Field("server", AccountInfoString(ACCOUNT_SERVER));
   Field("company", AccountInfoString(ACCOUNT_COMPANY));
   Field("currency", AccountInfoString(ACCOUNT_CURRENCY));
   FieldL("leverage", AccountInfoInteger(ACCOUNT_LEVERAGE));
   Field("trade_mode", TradeModeName());
   Field("margin_mode", MarginModeName());
   FieldD("balance", AccountInfoDouble(ACCOUNT_BALANCE));
   FieldD("equity", AccountInfoDouble(ACCOUNT_EQUITY));
   FieldD("profit", AccountInfoDouble(ACCOUNT_PROFIT));
   FieldD("margin", AccountInfoDouble(ACCOUNT_MARGIN));
   FieldD("margin_free", AccountInfoDouble(ACCOUNT_MARGIN_FREE));
   FieldL("gmt_offset", GmtOffset());
   FieldL("connected", TerminalInfoInteger(TERMINAL_CONNECTED));
   Field("trade_refusal", TradeRefusal());
   FieldL("build", TerminalInfoInteger(TERMINAL_BUILD));
   FieldL("magic", Magic);
   EndRecord();
   return Ok();
  }

//+------------------------------------------------------------------+
//| Market                                                           |
//+------------------------------------------------------------------+
string SymbolTradeModeName(const string sym)
  {
   long m = SymbolInfoInteger(sym, SYMBOL_TRADE_MODE);
   if(m == SYMBOL_TRADE_MODE_FULL)
      return "full";
   if(m == SYMBOL_TRADE_MODE_LONGONLY)
      return "longonly";
   if(m == SYMBOL_TRADE_MODE_SHORTONLY)
      return "shortonly";
   if(m == SYMBOL_TRADE_MODE_CLOSEONLY)
      return "closeonly";
   return "disabled";
  }

// SessionOpen reports whether now is inside one of the symbol's trading
// sessions of today (the broker's schedule; holidays are not in it).
bool SessionOpen(const string sym)
  {
   datetime now = TimeTradeServer();
   MqlDateTime dt;
   TimeToStruct(now, dt);
   long secs = dt.hour * 3600 + dt.min * 60 + dt.sec;
   datetime from, to;
   for(uint i = 0; SymbolInfoSessionTrade(sym, (ENUM_DAY_OF_WEEK)dt.day_of_week, i, from, to); i++)
      if(secs >= (long)from && secs < (long)to)
         return true;
   return false;
  }

string OpSymbol()
  {
   string sym = Arg("symbol");
   if(!SymbolSelect(sym, true))
      return Err("symbol", "unknown symbol " + sym + " on " + AccountInfoString(ACCOUNT_SERVER));
   Field("symbol", sym);
   FieldL("digits", SymbolInfoInteger(sym, SYMBOL_DIGITS));
   FieldD("point", SymbolInfoDouble(sym, SYMBOL_POINT));
   FieldD("tick_size", SymbolInfoDouble(sym, SYMBOL_TRADE_TICK_SIZE));
   FieldD("tick_value", SymbolInfoDouble(sym, SYMBOL_TRADE_TICK_VALUE));
   FieldD("contract", SymbolInfoDouble(sym, SYMBOL_TRADE_CONTRACT_SIZE));
   FieldD("vol_min", SymbolInfoDouble(sym, SYMBOL_VOLUME_MIN));
   FieldD("vol_max", SymbolInfoDouble(sym, SYMBOL_VOLUME_MAX));
   FieldD("vol_step", SymbolInfoDouble(sym, SYMBOL_VOLUME_STEP));
   Field("profit_ccy", SymbolInfoString(sym, SYMBOL_CURRENCY_PROFIT));
   Field("margin_ccy", SymbolInfoString(sym, SYMBOL_CURRENCY_MARGIN));
   Field("trade_mode", SymbolTradeModeName(sym));
   FieldL("stops_level", SymbolInfoInteger(sym, SYMBOL_TRADE_STOPS_LEVEL));
   FieldL("freeze_level", SymbolInfoInteger(sym, SYMBOL_TRADE_FREEZE_LEVEL));
   FieldL("session_open", SessionOpen(sym) ? 1 : 0);
   MqlTick t;
   if(SymbolInfoTick(sym, t))
     {
      FieldD("bid", t.bid);
      FieldD("ask", t.ask);
      FieldL("tick_time", t.time_msc / 1000 - GmtOffset());
     }
   EndRecord();
   return Ok();
  }

ENUM_TIMEFRAMES Timeframe(const string tf)
  {
   if(tf == "1m")  return PERIOD_M1;
   if(tf == "3m")  return PERIOD_M3;
   if(tf == "5m")  return PERIOD_M5;
   if(tf == "15m") return PERIOD_M15;
   if(tf == "30m") return PERIOD_M30;
   if(tf == "1h")  return PERIOD_H1;
   if(tf == "2h")  return PERIOD_H2;
   if(tf == "4h")  return PERIOD_H4;
   if(tf == "6h")  return PERIOD_H6;
   if(tf == "8h")  return PERIOD_H8;
   if(tf == "12h") return PERIOD_H12;
   if(tf == "1d")  return PERIOD_D1;
   if(tf == "1w")  return PERIOD_W1;
   return PERIOD_CURRENT;
  }

// OpRates gives candles oldest first: the last count, or those from from
// to to (UTC). The newest may still be forming.
string OpRates()
  {
   string sym = Arg("symbol");
   ENUM_TIMEFRAMES tf = Timeframe(Arg("tf"));
   if(tf == PERIOD_CURRENT)
      return Err("tf", "unknown timeframe " + Arg("tf"));
   if(!SymbolSelect(sym, true))
      return Err("symbol", "unknown symbol " + sym + " on " + AccountInfoString(ACCOUNT_SERVER));
   long from = ArgL("from");
   long to = ArgL("to");
   int count = (int)ArgL("count");
   MqlRates r[];
   int n = -1;
   // The terminal loads history on demand: the first ask can come back short.
   for(int attempt = 0; attempt < 30; attempt++)
     {
      if(from > 0)
         n = CopyRates(sym, tf, ServerTime(from), ServerTime(to), r);
      else
         n = CopyRates(sym, tf, 0, count, r);
      if(n > 0 && SeriesInfoInteger(sym, tf, SERIES_SYNCHRONIZED) != 0)
         break;
      Sleep(100);
     }
   if(n <= 0)
      return Err("history", StringFormat("no %s %s candles yet (error %d): the terminal is still loading its history, try again", sym, Arg("tf"), GetLastError()));
   long off = GmtOffset();
   for(int i = 0; i < n; i++)
     {
      FieldL("t", (long)r[i].time - off);
      FieldD("o", r[i].open);
      FieldD("h", r[i].high);
      FieldD("l", r[i].low);
      FieldD("c", r[i].close);
      FieldL("v", r[i].tick_volume);
      EndRecord();
     }
   return Ok();
  }

//+------------------------------------------------------------------+
//| Positions, orders, history                                       |
//+------------------------------------------------------------------+
string OrderTypeName(const long t)
  {
   switch((int)t)
     {
      case ORDER_TYPE_BUY:             return "buy";
      case ORDER_TYPE_SELL:            return "sell";
      case ORDER_TYPE_BUY_LIMIT:       return "buy_limit";
      case ORDER_TYPE_SELL_LIMIT:      return "sell_limit";
      case ORDER_TYPE_BUY_STOP:        return "buy_stop";
      case ORDER_TYPE_SELL_STOP:       return "sell_stop";
      case ORDER_TYPE_BUY_STOP_LIMIT:  return "buy_stop_limit";
      case ORDER_TYPE_SELL_STOP_LIMIT: return "sell_stop_limit";
      case ORDER_TYPE_CLOSE_BY:        return "close_by";
     }
   return "other";
  }

string OrderStateName(const long s)
  {
   switch((int)s)
     {
      case ORDER_STATE_STARTED:        return "started";
      case ORDER_STATE_PLACED:         return "placed";
      case ORDER_STATE_CANCELED:       return "canceled";
      case ORDER_STATE_PARTIAL:        return "partial";
      case ORDER_STATE_FILLED:         return "filled";
      case ORDER_STATE_REJECTED:       return "rejected";
      case ORDER_STATE_EXPIRED:        return "expired";
      case ORDER_STATE_REQUEST_ADD:    return "request_add";
      case ORDER_STATE_REQUEST_MODIFY: return "request_modify";
      case ORDER_STATE_REQUEST_CANCEL: return "request_cancel";
     }
   return "other";
  }

string OpPositions()
  {
   string sym = Arg("symbol");
   long off = GmtOffset();
   for(int i = 0; i < PositionsTotal(); i++)
     {
      ulong ticket = PositionGetTicket(i); // selects it
      if(ticket == 0 || (sym != "" && PositionGetString(POSITION_SYMBOL) != sym))
         continue;
      FieldL("ticket", (long)ticket);
      Field("symbol", PositionGetString(POSITION_SYMBOL));
      Field("type", PositionGetInteger(POSITION_TYPE) == POSITION_TYPE_BUY ? "buy" : "sell");
      FieldD("volume", PositionGetDouble(POSITION_VOLUME));
      FieldD("price_open", PositionGetDouble(POSITION_PRICE_OPEN));
      FieldD("price_current", PositionGetDouble(POSITION_PRICE_CURRENT));
      FieldD("sl", PositionGetDouble(POSITION_SL));
      FieldD("tp", PositionGetDouble(POSITION_TP));
      FieldD("profit", PositionGetDouble(POSITION_PROFIT));
      FieldD("swap", PositionGetDouble(POSITION_SWAP));
      FieldL("time", PositionGetInteger(POSITION_TIME) - off);
      FieldL("magic", PositionGetInteger(POSITION_MAGIC));
      EndRecord();
     }
   return Ok();
  }

string OpOrders()
  {
   string sym = Arg("symbol");
   long off = GmtOffset();
   for(int i = 0; i < OrdersTotal(); i++)
     {
      ulong ticket = OrderGetTicket(i); // selects it
      if(ticket == 0 || (sym != "" && OrderGetString(ORDER_SYMBOL) != sym))
         continue;
      FieldL("ticket", (long)ticket);
      Field("symbol", OrderGetString(ORDER_SYMBOL));
      Field("type", OrderTypeName(OrderGetInteger(ORDER_TYPE)));
      Field("state", OrderStateName(OrderGetInteger(ORDER_STATE)));
      FieldD("volume_initial", OrderGetDouble(ORDER_VOLUME_INITIAL));
      FieldD("volume_current", OrderGetDouble(ORDER_VOLUME_CURRENT));
      FieldD("price", OrderGetDouble(ORDER_PRICE_OPEN));
      FieldD("sl", OrderGetDouble(ORDER_SL));
      FieldD("tp", OrderGetDouble(ORDER_TP));
      FieldL("time_setup", OrderGetInteger(ORDER_TIME_SETUP) - off);
      FieldL("magic", OrderGetInteger(ORDER_MAGIC));
      EndRecord();
     }
   return Ok();
  }

// Fills sums the deals of an order: the volume filled, its average price,
// and the time of the last one (server time).
void Fills(const ulong order, double &volume, double &avg, long &last)
  {
   volume = 0;
   avg = 0;
   last = 0;
   if(!HistorySelect(0, TimeCurrent() + 86400))
      return;
   double value = 0;
   for(int i = 0; i < HistoryDealsTotal(); i++)
     {
      ulong d = HistoryDealGetTicket(i);
      if(d == 0 || HistoryDealGetInteger(d, DEAL_ORDER) != (long)order)
         continue;
      double v = HistoryDealGetDouble(d, DEAL_VOLUME);
      volume += v;
      value += v * HistoryDealGetDouble(d, DEAL_PRICE);
      long t = HistoryDealGetInteger(d, DEAL_TIME);
      if(t > last)
         last = t;
     }
   if(volume > 0)
      avg = value / volume;
  }

// OrderRecord writes the order, waiting or in the history; false when
// there is no such order.
bool OrderRecord(const ulong ticket)
  {
   long off = GmtOffset();
   double filled, avg;
   long last;
   if(OrderSelect(ticket))
     {
      string sym = OrderGetString(ORDER_SYMBOL);
      long type = OrderGetInteger(ORDER_TYPE);
      long state = OrderGetInteger(ORDER_STATE);
      double vi = OrderGetDouble(ORDER_VOLUME_INITIAL);
      double vc = OrderGetDouble(ORDER_VOLUME_CURRENT);
      double price = OrderGetDouble(ORDER_PRICE_OPEN);
      double sl = OrderGetDouble(ORDER_SL);
      double tp = OrderGetDouble(ORDER_TP);
      long setup = OrderGetInteger(ORDER_TIME_SETUP);
      long magic = OrderGetInteger(ORDER_MAGIC);
      Fills(ticket, filled, avg, last);
      FieldL("ticket", (long)ticket);
      Field("symbol", sym);
      Field("type", OrderTypeName(type));
      Field("state", OrderStateName(state));
      FieldD("volume_initial", vi);
      FieldD("volume_current", vc);
      FieldD("price", price);
      FieldD("sl", sl);
      FieldD("tp", tp);
      FieldL("time_setup", setup - off);
      FieldL("time_done", 0);
      FieldD("filled", filled);
      FieldD("avg_price", avg);
      FieldL("magic", magic);
      EndRecord();
      return true;
     }
   if(!HistorySelect(0, TimeCurrent() + 86400))
      return false;
   long state;
   if(!HistoryOrderGetInteger(ticket, ORDER_STATE, state))
      return false;
   string hsym = HistoryOrderGetString(ticket, ORDER_SYMBOL);
   long htype = HistoryOrderGetInteger(ticket, ORDER_TYPE);
   double hvi = HistoryOrderGetDouble(ticket, ORDER_VOLUME_INITIAL);
   double hvc = HistoryOrderGetDouble(ticket, ORDER_VOLUME_CURRENT);
   double hprice = HistoryOrderGetDouble(ticket, ORDER_PRICE_OPEN);
   double hsl = HistoryOrderGetDouble(ticket, ORDER_SL);
   double htp = HistoryOrderGetDouble(ticket, ORDER_TP);
   long hsetup = HistoryOrderGetInteger(ticket, ORDER_TIME_SETUP);
   long hdone = HistoryOrderGetInteger(ticket, ORDER_TIME_DONE);
   long hmagic = HistoryOrderGetInteger(ticket, ORDER_MAGIC);
   Fills(ticket, filled, avg, last);
   FieldL("ticket", (long)ticket);
   Field("symbol", hsym);
   Field("type", OrderTypeName(htype));
   Field("state", OrderStateName(state));
   FieldD("volume_initial", hvi);
   FieldD("volume_current", hvc);
   FieldD("price", hprice);
   FieldD("sl", hsl);
   FieldD("tp", htp);
   FieldL("time_setup", hsetup - off);
   FieldL("time_done", hdone > 0 ? hdone - off : 0);
   FieldD("filled", filled);
   FieldD("avg_price", avg);
   FieldL("magic", hmagic);
   EndRecord();
   return true;
  }

string OpOrder()
  {
   ulong ticket = (ulong)ArgL("ticket");
   if(!OrderRecord(ticket))
      return Err("no_order", "no order " + IntegerToString((long)ticket));
   return Ok();
  }

string DealTypeName(const long t)
  {
   if(t == DEAL_TYPE_BUY)
      return "buy";
   if(t == DEAL_TYPE_SELL)
      return "sell";
   return "other";
  }

string DealEntryName(const long e)
  {
   switch((int)e)
     {
      case DEAL_ENTRY_IN:     return "in";
      case DEAL_ENTRY_OUT:    return "out";
      case DEAL_ENTRY_INOUT:  return "inout";
      case DEAL_ENTRY_OUT_BY: return "out_by";
     }
   return "other";
  }

// OpDeals lists the symbol's deals from from to to (UTC; to 0 = now).
string OpDeals()
  {
   string sym = Arg("symbol");
   long from = ArgL("from");
   long to = ArgL("to");
   datetime end = to > 0 ? ServerTime(to) : TimeCurrent() + 86400;
   if(!HistorySelect(ServerTime(from), end))
      return Err("history", "cannot read the trade history");
   long off = GmtOffset();
   for(int i = 0; i < HistoryDealsTotal(); i++)
     {
      ulong d = HistoryDealGetTicket(i);
      if(d == 0 || HistoryDealGetString(d, DEAL_SYMBOL) != sym)
         continue;
      FieldL("ticket", (long)d);
      FieldL("order", HistoryDealGetInteger(d, DEAL_ORDER));
      FieldL("position", HistoryDealGetInteger(d, DEAL_POSITION_ID));
      FieldL("time", HistoryDealGetInteger(d, DEAL_TIME) - off);
      Field("type", DealTypeName(HistoryDealGetInteger(d, DEAL_TYPE)));
      Field("entry", DealEntryName(HistoryDealGetInteger(d, DEAL_ENTRY)));
      FieldD("volume", HistoryDealGetDouble(d, DEAL_VOLUME));
      FieldD("price", HistoryDealGetDouble(d, DEAL_PRICE));
      FieldD("profit", HistoryDealGetDouble(d, DEAL_PROFIT));
      FieldD("commission", HistoryDealGetDouble(d, DEAL_COMMISSION));
      FieldD("swap", HistoryDealGetDouble(d, DEAL_SWAP));
      FieldD("fee", HistoryDealGetDouble(d, DEAL_FEE));
      FieldL("magic", HistoryDealGetInteger(d, DEAL_MAGIC));
      EndRecord();
     }
   return Ok();
  }

//+------------------------------------------------------------------+
//| Trading                                                          |
//+------------------------------------------------------------------+
ENUM_ORDER_TYPE_FILLING Filling(const string sym)
  {
   long m = SymbolInfoInteger(sym, SYMBOL_FILLING_MODE);
   if((m & SYMBOL_FILLING_FOK) != 0)
      return ORDER_FILLING_FOK;
   if((m & SYMBOL_FILLING_IOC) != 0)
      return ORDER_FILLING_IOC;
   return ORDER_FILLING_RETURN;
  }

// Send sends the request; on a filling mode the server refuses, it tries
// the others. why says what went wrong.
bool Send(MqlTradeRequest &req, MqlTradeResult &res, string &why)
  {
   ZeroMemory(res);
   ResetLastError();
   OrderSend(req, res);
   if(res.retcode == TRADE_RETCODE_INVALID_FILL && (req.action == TRADE_ACTION_DEAL || req.action == TRADE_ACTION_PENDING))
     {
      ENUM_ORDER_TYPE_FILLING modes[3] = {ORDER_FILLING_FOK, ORDER_FILLING_IOC, ORDER_FILLING_RETURN};
      ENUM_ORDER_TYPE_FILLING tried = req.type_filling;
      for(int i = 0; i < 3 && res.retcode == TRADE_RETCODE_INVALID_FILL; i++)
        {
         if(modes[i] == tried)
            continue;
         req.type_filling = modes[i];
         ZeroMemory(res);
         ResetLastError();
         OrderSend(req, res);
        }
     }
   if(res.retcode == TRADE_RETCODE_DONE || res.retcode == TRADE_RETCODE_DONE_PARTIAL ||
      res.retcode == TRADE_RETCODE_PLACED || res.retcode == TRADE_RETCODE_NO_CHANGES)
      return true;
   if(res.retcode == 0)
      why = StringFormat("the order was not sent (error %d)", GetLastError());
   else
      why = StringFormat("%s (retcode %u)", res.comment, res.retcode);
   return false;
  }

string Code(const MqlTradeResult &res)
  {
   return res.retcode == 0 ? "send" : IntegerToString((long)res.retcode);
  }

double DealPrice(const MqlTradeResult &res)
  {
   if(res.price > 0 || res.deal == 0)
      return res.price;
   if(HistoryDealSelect(res.deal))
      return HistoryDealGetDouble(res.deal, DEAL_PRICE);
   return 0;
  }

// OpDeal buys or sells at market.
string OpDeal()
  {
   string why = TradeRefusal();
   if(why != "")
      return Err("refused", why);
   string sym = Arg("symbol");
   string side = Arg("side");
   if(side != "buy" && side != "sell")
      return Err("side", "side must be buy or sell, not " + side);
   if(!SymbolSelect(sym, true))
      return Err("symbol", "unknown symbol " + sym);
   MqlTick t;
   if(!SymbolInfoTick(sym, t))
      return Err("tick", "no price for " + sym);
   MqlTradeRequest req;
   MqlTradeResult res;
   ZeroMemory(req);
   req.action = TRADE_ACTION_DEAL;
   req.symbol = sym;
   req.volume = ArgD("volume");
   req.type = side == "buy" ? ORDER_TYPE_BUY : ORDER_TYPE_SELL;
   req.price = side == "buy" ? t.ask : t.bid;
   req.deviation = (ulong)DeviationPoints;
   req.magic = (ulong)Magic;
   req.comment = "autotrade";
   req.type_filling = Filling(sym);
   if(!Send(req, res, why))
      return Err(Code(res), why);
   FieldL("order", (long)res.order);
   FieldL("deal", (long)res.deal);
   FieldD("volume", res.volume);
   FieldD("price", DealPrice(res));
   FieldL("time", (long)TimeCurrent() - GmtOffset());
   EndRecord();
   return Ok();
  }

// OpClose closes up to volume lots (0 = all) of the symbol's positions on
// the side opposite to side: side=sell closes longs.
string OpClose()
  {
   string why = TradeRefusal();
   if(why != "")
      return Err("refused", why);
   string sym = Arg("symbol");
   string side = Arg("side");
   double want = ArgD("volume");
   long closes = side == "sell" ? POSITION_TYPE_BUY : POSITION_TYPE_SELL;
   ulong tickets[];
   for(int i = 0; i < PositionsTotal(); i++)
     {
      ulong tk = PositionGetTicket(i);
      if(tk == 0 || PositionGetString(POSITION_SYMBOL) != sym || PositionGetInteger(POSITION_TYPE) != closes)
         continue;
      int n = ArraySize(tickets);
      ArrayResize(tickets, n + 1);
      tickets[n] = tk;
     }
   if(ArraySize(tickets) == 0)
      return Err("no_position", "no open " + (closes == POSITION_TYPE_BUY ? "long" : "short") + " position on " + sym);
   double closed = 0, value = 0;
   ulong last_order = 0;
   for(int i = 0; i < ArraySize(tickets); i++)
     {
      if(!PositionSelectByTicket(tickets[i]))
         continue;
      double vol = PositionGetDouble(POSITION_VOLUME);
      if(want > 0)
         vol = MathMin(vol, NormalizeDouble(want - closed, 8));
      if(vol <= 0)
         break;
      MqlTick t;
      if(!SymbolInfoTick(sym, t))
         return Err("tick", "no price for " + sym);
      bool isLong = closes == POSITION_TYPE_BUY;
      MqlTradeRequest req;
      MqlTradeResult res;
      ZeroMemory(req);
      req.action = TRADE_ACTION_DEAL;
      req.symbol = sym;
      req.volume = vol;
      req.type = isLong ? ORDER_TYPE_SELL : ORDER_TYPE_BUY;
      req.price = isLong ? t.bid : t.ask;
      req.deviation = (ulong)DeviationPoints;
      req.magic = (ulong)Magic;
      req.comment = "autotrade close";
      req.type_filling = Filling(sym);
      if(Hedging())
         req.position = tickets[i];
      if(!Send(req, res, why))
        {
         if(closed > 0)
            why = StringFormat("%s, after closing %.2f lots", why, closed);
         return Err(Code(res), why);
        }
      double px = DealPrice(res);
      closed += res.volume;
      value += res.volume * px;
      last_order = res.order;
     }
   FieldL("order", (long)last_order);
   FieldD("volume", closed);
   FieldD("price", closed > 0 ? value / closed : 0);
   FieldL("time", (long)TimeCurrent() - GmtOffset());
   EndRecord();
   return Ok();
  }

// OpLimit places a buy or sell limit order with its stop-loss and
// take-profit, which become the position's when it fills.
string OpLimit()
  {
   string why = TradeRefusal();
   if(why != "")
      return Err("refused", why);
   string sym = Arg("symbol");
   string side = Arg("side");
   if(side != "buy" && side != "sell")
      return Err("side", "side must be buy or sell, not " + side);
   if(!SymbolSelect(sym, true))
      return Err("symbol", "unknown symbol " + sym);
   int digits = (int)SymbolInfoInteger(sym, SYMBOL_DIGITS);
   MqlTradeRequest req;
   MqlTradeResult res;
   ZeroMemory(req);
   req.action = TRADE_ACTION_PENDING;
   req.symbol = sym;
   req.volume = ArgD("volume");
   req.type = side == "buy" ? ORDER_TYPE_BUY_LIMIT : ORDER_TYPE_SELL_LIMIT;
   req.price = NormalizeDouble(ArgD("price"), digits);
   req.sl = NormalizeDouble(ArgD("sl"), digits);
   req.tp = NormalizeDouble(ArgD("tp"), digits);
   req.type_time = ORDER_TIME_GTC;
   req.type_filling = ORDER_FILLING_RETURN;
   req.magic = (ulong)Magic;
   req.comment = "autotrade";
   if(!Send(req, res, why))
      return Err(Code(res), why);
   FieldL("order", (long)res.order);
   FieldL("time", (long)TimeCurrent() - GmtOffset());
   EndRecord();
   return Ok();
  }

// OpSLTP sets the stop-loss and take-profit (0 = none) of the symbol's
// positions, or of the one with ticket.
string OpSLTP()
  {
   string why = TradeRefusal();
   if(why != "")
      return Err("refused", why);
   string sym = Arg("symbol");
   ulong only = (ulong)ArgL("ticket");
   int digits = (int)SymbolInfoInteger(sym, SYMBOL_DIGITS);
   double sl = NormalizeDouble(ArgD("sl"), digits);
   double tp = NormalizeDouble(ArgD("tp"), digits);
   ulong tickets[];
   for(int i = 0; i < PositionsTotal(); i++)
     {
      ulong tk = PositionGetTicket(i);
      if(tk == 0 || PositionGetString(POSITION_SYMBOL) != sym || (only != 0 && tk != only))
         continue;
      int n = ArraySize(tickets);
      ArrayResize(tickets, n + 1);
      tickets[n] = tk;
     }
   if(ArraySize(tickets) == 0)
      return Err("no_position", "no open position on " + sym);
   for(int i = 0; i < ArraySize(tickets); i++)
     {
      MqlTradeRequest req;
      MqlTradeResult res;
      ZeroMemory(req);
      req.action = TRADE_ACTION_SLTP;
      req.symbol = sym;
      req.position = tickets[i];
      req.sl = sl;
      req.tp = tp;
      req.magic = (ulong)Magic;
      if(!Send(req, res, why))
         return Err(Code(res), why);
      FieldL("ticket", (long)tickets[i]);
      FieldD("sl", sl);
      FieldD("tp", tp);
      EndRecord();
     }
   return Ok();
  }

// OpModify sets a waiting order's stop-loss and take-profit (0 = none).
string OpModify()
  {
   string why = TradeRefusal();
   if(why != "")
      return Err("refused", why);
   ulong ticket = (ulong)ArgL("ticket");
   if(!OrderSelect(ticket))
      return Err("no_order", "no waiting order " + IntegerToString((long)ticket));
   string sym = OrderGetString(ORDER_SYMBOL);
   int digits = (int)SymbolInfoInteger(sym, SYMBOL_DIGITS);
   MqlTradeRequest req;
   MqlTradeResult res;
   ZeroMemory(req);
   req.action = TRADE_ACTION_MODIFY;
   req.order = ticket;
   req.symbol = sym;
   req.price = OrderGetDouble(ORDER_PRICE_OPEN);
   req.sl = NormalizeDouble(ArgD("sl"), digits);
   req.tp = NormalizeDouble(ArgD("tp"), digits);
   req.type_time = (ENUM_ORDER_TYPE_TIME)OrderGetInteger(ORDER_TYPE_TIME);
   req.expiration = (datetime)OrderGetInteger(ORDER_TIME_EXPIRATION);
   if(!Send(req, res, why))
      return Err(Code(res), why);
   FieldL("ticket", (long)ticket);
   FieldD("sl", req.sl);
   FieldD("tp", req.tp);
   EndRecord();
   return Ok();
  }

// OpCancel removes a waiting order and gives it as it ended.
string OpCancel()
  {
   string why = TradeRefusal();
   if(why != "")
      return Err("refused", why);
   ulong ticket = (ulong)ArgL("ticket");
   if(!OrderSelect(ticket))
      return Err("no_order", "no waiting order " + IntegerToString((long)ticket) + ": it filled or ended");
   MqlTradeRequest req;
   MqlTradeResult res;
   ZeroMemory(req);
   req.action = TRADE_ACTION_REMOVE;
   req.order = ticket;
   if(!Send(req, res, why))
      return Err(Code(res), why);
   // The history can take a moment to show the cancelled order.
   for(int attempt = 0; attempt < 20; attempt++)
     {
      if(!OrderSelect(ticket) && OrderRecord(ticket))
         return Ok();
      g_out = "";
      g_rec = "";
      Sleep(50);
     }
   double filled, avg;
   long last;
   Fills(ticket, filled, avg, last);
   FieldL("ticket", (long)ticket);
   Field("state", "canceled");
   FieldD("filled", filled);
   FieldD("avg_price", avg);
   EndRecord();
   return Ok();
  }
//+------------------------------------------------------------------+
