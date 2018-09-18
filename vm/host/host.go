package host

import (
	"strconv"

	"encoding/json"

	"fmt"

	"github.com/iost-official/Go-IOS-Protocol/core/contract"
	"github.com/iost-official/Go-IOS-Protocol/core/tx"
	"github.com/iost-official/Go-IOS-Protocol/ilog"
	"github.com/iost-official/Go-IOS-Protocol/vm/database"
)

// Monitor monitor interface
type Monitor interface {
	Call(host *Host, contractName, api string, jarg string) (rtn []interface{}, cost *contract.Cost, err error)
	Compile(con *contract.Contract) (string, error)
}

// Host host struct, used as isolate of vm
type Host struct {
	DBHandler
	Info
	Teller
	APIDelegate
	EventPoster
	DHCP

	logger  *ilog.Logger
	ctx     *Context
	db      *database.Visitor
	monitor Monitor
}

// NewHost get a new host
func NewHost(ctx *Context, db *database.Visitor, monitor Monitor, logger *ilog.Logger) *Host {
	h := &Host{
		ctx:     ctx,
		db:      db,
		monitor: monitor,
		logger:  logger,
	}
	h.DBHandler = NewDBHandler(h)
	h.Info = NewInfo(h)
	h.Teller = NewTeller(h)
	h.APIDelegate = NewAPI(h)
	h.EventPoster = EventPoster{}
	h.DHCP = NewDHCP(h)

	return h

}

// Context get context in host
func (h *Host) Context() *Context {
	return h.ctx
}

// SetContext set a new context to host
func (h *Host) SetContext(ctx *Context) {
	h.ctx = ctx

}

// Call  call a new contract in this context
func (h *Host) Call(contract, api, jarg string) ([]interface{}, *contract.Cost, error) {

	// save stack
	record := contract + "-" + api

	height := h.ctx.Value("stack_height").(int)

	for i := 0; i < height; i++ {
		key := "stack" + strconv.Itoa(i)
		if h.ctx.Value(key).(string) == record {
			return nil, nil, ErrReenter
		}
	}

	key := "stack" + strconv.Itoa(height)

	h.ctx = NewContext(h.ctx)

	h.ctx.Set("stack_height", height+1)
	h.ctx.Set(key, record)
	rtn, cost, err := h.monitor.Call(h, contract, api, jarg)

	h.ctx = h.ctx.Base()

	return rtn, cost, err
}

// CallWithReceipt call and generate receipt
func (h *Host) CallWithReceipt(contractName, api, jarg string) ([]interface{}, *contract.Cost, error) {
	rtn, cost, err := h.Call(contractName, api, jarg)

	var sarr []interface{}
	sarr = append(sarr, api)
	sarr = append(sarr, jarg)

	if err != nil {
		sarr = append(sarr, err.Error())
	} else {
		sarr = append(sarr, "success")
	}
	s, err := json.Marshal(sarr)
	if err != nil {
		return rtn, cost, err
	}
	h.receipt(tx.SystemDefined, string(s))
	cost.AddAssign(ReceiptCost(len(s)))
	return rtn, cost, err

}

// SetCode set code to storage
func (h *Host) SetCode(c *contract.Contract) (*contract.Cost, error) {
	code, err := h.monitor.Compile(c)
	if err != nil {
		return CompileErrCost, err
	}
	c.Code = code

	initABI := contract.ABI{
		Name:    "init",
		Payment: 0,
		Args:    []string{},
	}

	c.Info.Abis = append(c.Info.Abis, &initABI)

	l := len(c.Encode()) // todo multi Encode call
	//ilog.Debugf("length is : %v", l)

	h.db.SetContract(c)

	_, cost, err := h.monitor.Call(h, c.ID, "init", "[]")

	cost.AddAssign(CodeSavageCost(l))

	//ilog.Debugf("set gas is : %v", cost.ToGas())

	return cost, err // todo check set cost
}

// UpdateCode update code
func (h *Host) UpdateCode(c *contract.Contract, id database.SerializedJSON) (*contract.Cost, error) {
	oc := h.db.Contract(c.ID)
	if oc == nil {
		return ContractNotFoundCost, ErrContractNotFound
	}
	abi := oc.ABI("can_update")
	if abi == nil {
		return ABINotFoundCost, ErrUpdateRefused
	}

	rtn, cost, err := h.monitor.Call(h, c.ID, "can_update", `["`+string(id)+`"]`)

	if err != nil {
		return cost, fmt.Errorf("call can_update: %v", err)
	}

	// todo rtn[0] should be bool type
	if t, ok := rtn[0].(string); !ok || t != "true" {
		return cost, ErrUpdateRefused
	}

	c2, err := h.SetCode(c)

	c2.AddAssign(cost)
	return c2, err
}

// DestroyCode delete code
func (h *Host) DestroyCode(contractName string) (*contract.Cost, error) {
	// todo free kv

	oc := h.db.Contract(contractName)
	if oc == nil {
		return ContractNotFoundCost, ErrContractNotFound
	}
	abi := oc.ABI("can_destroy")
	if abi == nil {
		return ABINotFoundCost, ErrDestroyRefused
	}

	rtn, cost, err := h.monitor.Call(h, contractName, "can_destroy", "[]")

	if err != nil {
		return cost, err
	}

	// todo rtn[0] should be bool type
	if t, ok := rtn[0].(string); !ok || t != "true" {
		return cost, ErrDestroyRefused
	}

	h.db.DelContract(contractName)
	return DelContractCost, nil
}

// Logger get a log in host
func (h *Host) Logger() *ilog.Logger {
	return h.logger
}

// DB get current version mvccdb
func (h *Host) DB() *database.Visitor {
	return h.db
}

// PushCtx make a new context based on current one
func (h *Host) PushCtx() {
	ctx := NewContext(h.ctx)
	h.ctx = ctx
}

// PopCtx pop current context
func (h *Host) PopCtx() {
	ctx := h.ctx.Base()
	h.ctx = ctx
}
