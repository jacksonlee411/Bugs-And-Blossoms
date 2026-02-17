import { describe, expect, it } from "vitest"
import { planRecordEffectiveDate, validatePlannedEffectiveDate } from "./orgUnitRecordDateRules"

describe("orgUnitRecordDateRules", () => {
  it("add mode defaults to last+1 and enforces lower bound", () => {
    const plan = planRecordEffectiveDate({
      mode: "add",
      selectedEffectiveDate: "2026-02-01",
      versions: [{ effective_date: "2026-01-01" }, { effective_date: "2026-02-01" }]
    })

    expect(plan.kind).toBe("add")
    expect(plan.defaultDate).toBe("2026-02-02")
    expect(plan.minDate).toBe("2026-02-02")
    expect(validatePlannedEffectiveDate({ plan, effectiveDate: "2026-02-01" })).toEqual({
      ok: false,
      reason: "out_of_range"
    })
    expect(validatePlannedEffectiveDate({ plan, effectiveDate: "2026-02-02" })).toEqual({ ok: true })
  })

  it("insert mode on middle version returns bounded range", () => {
    const plan = planRecordEffectiveDate({
      mode: "insert",
      selectedEffectiveDate: "2026-02-01",
      versions: [
        { effective_date: "2026-01-01" },
        { effective_date: "2026-02-01" },
        { effective_date: "2026-02-10" }
      ]
    })

    expect(plan.kind).toBe("insert")
    expect(plan.minDate).toBe("2026-01-02")
    expect(plan.maxDate).toBe("2026-02-09")
    expect(plan.defaultDate).toBe("2026-02-02")
    expect(validatePlannedEffectiveDate({ plan, effectiveDate: "2026-02-10" })).toEqual({
      ok: false,
      reason: "out_of_range"
    })
    expect(validatePlannedEffectiveDate({ plan, effectiveDate: "2026-02-03" })).toEqual({ ok: true })
  })

  it("insert on earliest version follows frozen min=selected+1 rule", () => {
    const plan = planRecordEffectiveDate({
      mode: "insert",
      selectedEffectiveDate: "2026-01-01",
      versions: [{ effective_date: "2026-01-01" }, { effective_date: "2026-02-01" }]
    })

    expect(plan.kind).toBe("insert")
    expect(plan.minDate).toBe("2026-01-02")
    expect(plan.maxDate).toBe("2026-01-31")
  })

  it("insert on latest version degrades to insert_as_add", () => {
    const plan = planRecordEffectiveDate({
      mode: "insert",
      selectedEffectiveDate: "2026-02-01",
      versions: [{ effective_date: "2026-01-01" }, { effective_date: "2026-02-01" }]
    })

    expect(plan.kind).toBe("insert_as_add")
    expect(plan.minDate).toBe("2026-02-02")
    expect(plan.maxDate).toBeNull()
    expect(validatePlannedEffectiveDate({ plan, effectiveDate: "2026-02-01" })).toEqual({
      ok: false,
      reason: "out_of_range"
    })
  })

  it("insert with no slot returns insert_no_slot and blocks submit", () => {
    const plan = planRecordEffectiveDate({
      mode: "insert",
      selectedEffectiveDate: "2026-01-01",
      versions: [{ effective_date: "2026-01-01" }, { effective_date: "2026-01-02" }]
    })

    expect(plan.kind).toBe("insert_no_slot")
    expect(validatePlannedEffectiveDate({ plan, effectiveDate: "2026-01-02" })).toEqual({
      ok: false,
      reason: "no_slot"
    })
  })
})
