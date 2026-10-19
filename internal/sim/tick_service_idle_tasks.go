package sim

// Port of reference/tower-together/sim/tick/service-idle-tasks.ts.
//
// 1268:01a6 service_idle_tasks: the Win16 idle pass calls
// run_simulation_day_scheduler then carrier_tick. Entry point for
// TowerSim.Step.

// ServiceIdleTasksContext is the mutable context passed into
// ServiceIdleTasks. ServiceIdleTasks replaces ctx.Time with the advanced
// *TimeState (the TS assigns a fresh object) before CarrierTick runs, and
// TowerSim.Step reads it back.
// TS: tick/service-idle-tasks.ts ServiceIdleTasksContext
type ServiceIdleTasksContext struct {
	World  *WorldState
	Ledger *LedgerState
	Time   *TimeState
}

// ServiceIdleTasks TS: tick/service-idle-tasks.ts serviceIdleTasks (binary 1268:01a6)
func ServiceIdleTasks(ctx *ServiceIdleTasksContext) {
	ctx.Time = RunSimulationDayScheduler(ctx.World, ctx.Ledger, ctx.Time)
	CarrierTick(ctx.World, ctx.Ledger, ctx.Time)
}
