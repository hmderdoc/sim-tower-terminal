package sim

// Port of reference/tower-together/sim/families/medical.ts.
//
// The TS file only re-exports sims/medical.ts (handleMedicalSimArrival,
// invalidateMedicalSlotsForSidecar, MEDICAL_NOTIFICATION_MESSAGE,
// processMedicalSim, STATE_MEDICAL_*, tryStartMedicalTrip). In the flat Go
// package those names live in sims_medical.go, so there is nothing to
// declare here. The TS notes no 1228:* function has been mapped to the
// medical subsystem yet.
