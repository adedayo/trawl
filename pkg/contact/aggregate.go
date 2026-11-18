package contact

// WeaknessInstance represents one inventory instance of a weakness class and
// whether re-observation has confirmed the weakness absent on that instance.
type WeaknessInstance struct {
	InstanceID          string               `json:"instanceId"`
	AssetID             string               `json:"assetId"`
	ServiceID           string               `json:"serviceId"`
	ConfirmedRemediated bool                 `json:"confirmedRemediated"`
	Estimate            ContactPriorEstimate `json:"estimate"`
}

// InstanceAggregation summarises heterogeneous contact probability across all
// instances of a weakness alongside confirmed remediation coverage and residual
// contact probability over uncovered instances.
type InstanceAggregation struct {
	WeaknessClass        string           `json:"weaknessClass"`
	TotalInstances       int              `json:"totalInstances"`
	ConfirmedRemediated  int              `json:"confirmedRemediated"`
	UncoveredInstances   int              `json:"uncoveredInstances"`
	UnknownReachability  int              `json:"unknownReachability"`
	Available            bool             `json:"available"`
	AllInstancesPContact Interval         `json:"allInstancesPContact"`
	ResidualPContact     Interval         `json:"residualPContact"`
	Coverage             CoverageFigure   `json:"coverage"`
	Label                CalibrationLabel `json:"label"`
	ScopeStatement       string           `json:"scopeStatement"`
	IsPrior              bool             `json:"isPrior"`
}

// AggregateInstances computes 1 - prod_i (1 - P_i) across heterogeneous
// instances (reducing exactly to 1 - (1 - P)^N when homogeneous) and computes
// the residual estimate strictly over instances not confirmed remediated on
// re-observation.
func AggregateInstances(weaknessClass string, instances []WeaknessInstance) InstanceAggregation {
	agg := InstanceAggregation{
		WeaknessClass:  weaknessClass,
		TotalInstances: len(instances),
		Label:          LabelSourced,
		ScopeStatement: AmbientScopeStatement,
		IsPrior:        true,
	}
	if len(instances) == 0 {
		return agg
	}

	compAllPoint, compAllLow, compAllHigh := 1.0, 1.0, 1.0
	compResPoint, compResLow, compResHigh := 1.0, 1.0, 1.0
	anyAvailable := false

	for _, inst := range instances {
		agg.Coverage.ChecksConcluded += inst.Estimate.Coverage.ChecksConcluded
		agg.Coverage.ChecksApplicable += inst.Estimate.Coverage.ChecksApplicable

		if inst.ConfirmedRemediated {
			agg.ConfirmedRemediated++
		} else {
			agg.UncoveredInstances++
		}

		if !inst.Estimate.Available {
			agg.UnknownReachability++
			continue
		}
		anyAvailable = true
		if inst.Estimate.Label == LabelIllustrative {
			agg.Label = LabelIllustrative
		} else if inst.Estimate.Label == LabelCalibrated && agg.Label != LabelIllustrative {
			agg.Label = LabelCalibrated
		}

		compAllPoint *= (1.0 - clampProb(inst.Estimate.PContact.Point))
		compAllLow *= (1.0 - clampProb(inst.Estimate.PContact.Low))
		compAllHigh *= (1.0 - clampProb(inst.Estimate.PContact.High))

		if !inst.ConfirmedRemediated {
			compResPoint *= (1.0 - clampProb(inst.Estimate.PContact.Point))
			compResLow *= (1.0 - clampProb(inst.Estimate.PContact.Low))
			compResHigh *= (1.0 - clampProb(inst.Estimate.PContact.High))
		}
	}

	if !anyAvailable {
		return agg
	}

	agg.Available = true
	agg.AllInstancesPContact = Interval{
		Point: 1.0 - compAllPoint,
		Low:   1.0 - compAllLow,
		High:  1.0 - compAllHigh,
	}
	agg.ResidualPContact = Interval{
		Point: 1.0 - compResPoint,
		Low:   1.0 - compResLow,
		High:  1.0 - compResHigh,
	}
	return agg
}

func clampProb(p float64) float64 {
	if p < 0 {
		return 0
	}
	if p > 1 {
		return 1
	}
	return p
}
