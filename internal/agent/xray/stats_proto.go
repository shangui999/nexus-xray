package xray

import (
	"fmt"

	"google.golang.org/protobuf/encoding/protowire"
)

// 以下为 Xray Stats gRPC API 的手写 protobuf 消息定义
// 参考 xray-core 的 app/stats/command/command.proto

// GetStatsRequest 对应 proto: message GetStatsRequest { string name = 1; bool reset = 2; }
type GetStatsRequest struct {
	Name  string
	Reset bool
}

// GetStatsResponse 对应 proto: message GetStatsResponse { Stat stat = 1; }
type GetStatsResponse struct {
	Stat *Stat
}

// QueryStatsRequest 对应 proto: message QueryStatsRequest { string pattern = 1; bool reset = 2; }
type QueryStatsRequest struct {
	Pattern string
	Reset   bool
}

// QueryStatsResponse 对应 proto: message QueryStatsResponse { repeated Stat stat = 1; }
type QueryStatsResponse struct {
	Stats []*Stat
}

// Stat 对应 proto: message Stat { string name = 1; int64 value = 2; }
type Stat struct {
	Name  string
	Value int64
}

// --- 手动 protobuf 编码/解码 ---

func marshalStat(s *Stat) []byte {
	b := []byte{}
	if s.Name != "" {
		b = protowire.AppendTag(b, 1, protowire.BytesType)
		b = protowire.AppendString(b, s.Name)
	}
	if s.Value != 0 {
		b = protowire.AppendTag(b, 2, protowire.VarintType)
		b = protowire.AppendVarint(b, uint64(s.Value))
	}
	return b
}

func unmarshalStat(data []byte) (*Stat, error) {
	s := &Stat{}
	for len(data) > 0 {
		num, typ, n := protowire.ConsumeTag(data)
		if n < 0 {
			return nil, fmt.Errorf("consume tag: %v", protowire.ParseError(n))
		}
		data = data[n:]

		switch num {
		case 1:
			if typ != protowire.BytesType {
				return nil, fmt.Errorf("unexpected wire type for field name: %d", typ)
			}
			v, n := protowire.ConsumeString(data)
			if n < 0 {
				return nil, fmt.Errorf("consume string: %v", protowire.ParseError(n))
			}
			data = data[n:]
			s.Name = v
		case 2:
			if typ != protowire.VarintType {
				return nil, fmt.Errorf("unexpected wire type for field value: %d", typ)
			}
			v, n := protowire.ConsumeVarint(data)
			if n < 0 {
				return nil, fmt.Errorf("consume varint: %v", protowire.ParseError(n))
			}
			data = data[n:]
			s.Value = int64(v)
		default:
			// 跳过未知字段
			n := protowire.ConsumeFieldValue(num, typ, data)
			if n < 0 {
				return nil, fmt.Errorf("consume field value: %v", protowire.ParseError(n))
			}
			data = data[n:]
		}
	}
	return s, nil
}

func marshalGetStatsRequest(req *GetStatsRequest) []byte {
	b := []byte{}
	if req.Name != "" {
		b = protowire.AppendTag(b, 1, protowire.BytesType)
		b = protowire.AppendString(b, req.Name)
	}
	if req.Reset {
		b = protowire.AppendTag(b, 2, protowire.VarintType)
		b = protowire.AppendVarint(b, 1)
	}
	return b
}

func unmarshalGetStatsResponse(data []byte) (*GetStatsResponse, error) {
	resp := &GetStatsResponse{}
	for len(data) > 0 {
		num, typ, n := protowire.ConsumeTag(data)
		if n < 0 {
			return nil, fmt.Errorf("consume tag: %v", protowire.ParseError(n))
		}
		data = data[n:]

		switch num {
		case 1:
			if typ != protowire.BytesType {
				return nil, fmt.Errorf("unexpected wire type for field stat: %d", typ)
			}
			v, n := protowire.ConsumeBytes(data)
			if n < 0 {
				return nil, fmt.Errorf("consume bytes: %v", protowire.ParseError(n))
			}
			data = data[n:]
			s, err := unmarshalStat(v)
			if err != nil {
				return nil, fmt.Errorf("unmarshal stat: %w", err)
			}
			resp.Stat = s
		default:
			n := protowire.ConsumeFieldValue(num, typ, data)
			if n < 0 {
				return nil, fmt.Errorf("consume field value: %v", protowire.ParseError(n))
			}
			data = data[n:]
		}
	}
	return resp, nil
}

func marshalQueryStatsRequest(req *QueryStatsRequest) []byte {
	b := []byte{}
	if req.Pattern != "" {
		b = protowire.AppendTag(b, 1, protowire.BytesType)
		b = protowire.AppendString(b, req.Pattern)
	}
	if req.Reset {
		b = protowire.AppendTag(b, 2, protowire.VarintType)
		b = protowire.AppendVarint(b, 1)
	}
	return b
}

func unmarshalQueryStatsResponse(data []byte) (*QueryStatsResponse, error) {
	resp := &QueryStatsResponse{}
	for len(data) > 0 {
		num, typ, n := protowire.ConsumeTag(data)
		if n < 0 {
			return nil, fmt.Errorf("consume tag: %v", protowire.ParseError(n))
		}
		data = data[n:]

		switch num {
		case 1:
			if typ != protowire.BytesType {
				return nil, fmt.Errorf("unexpected wire type for field stat: %d", typ)
			}
			v, n := protowire.ConsumeBytes(data)
			if n < 0 {
				return nil, fmt.Errorf("consume bytes: %v", protowire.ParseError(n))
			}
			data = data[n:]
			s, err := unmarshalStat(v)
			if err != nil {
				return nil, fmt.Errorf("unmarshal stat: %w", err)
			}
			resp.Stats = append(resp.Stats, s)
		default:
			n := protowire.ConsumeFieldValue(num, typ, data)
			if n < 0 {
				return nil, fmt.Errorf("consume field value: %v", protowire.ParseError(n))
			}
			data = data[n:]
		}
	}
	return resp, nil
}
