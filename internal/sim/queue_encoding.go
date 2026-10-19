package sim

// Port of reference/tower-together/sim/queue/encoding.ts.
//
// 1218:1b96 decode_runtime_route_target
//
// Decode the byte at sim+0x08 (encoded_route_target) into (slot, direction):
//
//	byte < 0x40   -> special-link slot, slot = byte
//	0x40..0x57   -> carrier slot, direction = up,   slot = byte - 0x40
//	0x58..0x6f   -> carrier slot, direction = down, slot = byte - 0x58

const (
	RouteTargetSpecialLinkMax  = 0x40
	RouteTargetCarrierUpBase   = 0x40
	RouteTargetCarrierDownBase = 0x58
)

// RuntimeRouteTarget kinds.
const (
	RouteTargetKindSpecialLink = "special-link"
	RouteTargetKindCarrier     = "carrier"
)

// RuntimeRouteTarget TS: queue/encoding.ts RuntimeRouteTarget
type RuntimeRouteTarget struct {
	// Kind: "special-link" = stairs/escalator segment; "carrier" = elevator.
	Kind string
	// Slot: segment index or carrier index, depending on Kind.
	Slot int
	// DirectionFlag: 1 = up, 0 = down. Always 0 for special-link targets.
	DirectionFlag int
}

// DecodeEncodedRouteTargetByte decodes a raw encoded-route-target byte; nil
// for 0xff (unrouted).
// TS: queue/encoding.ts decodeEncodedRouteTargetByte
func DecodeEncodedRouteTargetByte(b int) *RuntimeRouteTarget {
	if b == 0xff {
		return nil
	}
	if b < RouteTargetSpecialLinkMax {
		return &RuntimeRouteTarget{Kind: RouteTargetKindSpecialLink, Slot: b, DirectionFlag: 0}
	}
	if b < RouteTargetCarrierDownBase {
		return &RuntimeRouteTarget{Kind: RouteTargetKindCarrier, Slot: b - RouteTargetCarrierUpBase, DirectionFlag: 1}
	}
	return &RuntimeRouteTarget{Kind: RouteTargetKindCarrier, Slot: b - RouteTargetCarrierDownBase, DirectionFlag: 0}
}

// EncodeRuntimeRouteTarget encodes a target into the binary's one-byte form.
// TS: queue/encoding.ts encodeRuntimeRouteTarget
func EncodeRuntimeRouteTarget(target RuntimeRouteTarget) int {
	if target.Kind == RouteTargetKindSpecialLink {
		return target.Slot & 0xff
	}
	base := RouteTargetCarrierDownBase
	if target.DirectionFlag == 1 {
		base = RouteTargetCarrierUpBase
	}
	return (base + target.Slot) & 0xff
}

// DecodeRuntimeRouteTarget reads the sim's route state (sim.Route) and
// returns the decoded view, or nil when idle.
// TS: queue/encoding.ts decodeRuntimeRouteTarget (binary 1218:1b96)
func DecodeRuntimeRouteTarget(sim *SimRecord) *RuntimeRouteTarget {
	if sim.Route.Mode == RouteModeCarrier {
		dir := 0
		if sim.Route.Direction == "up" {
			dir = 1
		}
		return &RuntimeRouteTarget{Kind: RouteTargetKindCarrier, Slot: sim.Route.CarrierID, DirectionFlag: dir}
	}
	if sim.Route.Mode == RouteModeSegment {
		return &RuntimeRouteTarget{Kind: RouteTargetKindSpecialLink, Slot: sim.Route.SegmentID, DirectionFlag: 0}
	}
	return nil
}
