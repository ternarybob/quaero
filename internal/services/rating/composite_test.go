package rating

import "testing"

func TestCalculateRating(t *testing.T) {
	tests := []struct {
		name           string
		bfs            BFSResult
		cds            CDSResult
		nfr            NFRResult
		pps            PPSResult
		vrs            VRSResult
		ob             OBResult
		wantGatePassed bool
		wantLabel      RatingLabel
		wantInvestNil  bool
	}{
		{
			name:           "gate fails - BFS too low",
			bfs:            BFSResult{Score: 0},
			cds:            CDSResult{Score: 2},
			nfr:            NFRResult{Score: 0.8},
			pps:            PPSResult{Score: 0.7},
			vrs:            VRSResult{Score: 0.6},
			ob:             OBResult{Score: 1.0},
			wantGatePassed: false,
			wantLabel:      LabelSpeculative,
			wantInvestNil:  true,
		},
		{
			name:           "gate fails - CDS too low",
			bfs:            BFSResult{Score: 2},
			cds:            CDSResult{Score: 0},
			nfr:            NFRResult{Score: 0.8},
			pps:            PPSResult{Score: 0.7},
			vrs:            VRSResult{Score: 0.6},
			ob:             OBResult{Score: 1.0},
			wantGatePassed: false,
			wantLabel:      LabelSpeculative,
			wantInvestNil:  true,
		},
		{
			name:           "gate passes - minimum scores",
			bfs:            BFSResult{Score: 1},
			cds:            CDSResult{Score: 1},
			nfr:            NFRResult{Score: 0.5},
			pps:            PPSResult{Score: 0.5},
			vrs:            VRSResult{Score: 0.5},
			ob:             OBResult{Score: 0},
			wantGatePassed: true,
			wantLabel:      LabelLowAlpha,
			wantInvestNil:  false,
		},
		{
			name:           "high conviction - all scores high",
			bfs:            BFSResult{Score: 2},
			cds:            CDSResult{Score: 2},
			nfr:            NFRResult{Score: 1.0},
			pps:            PPSResult{Score: 1.0},
			vrs:            VRSResult{Score: 1.0},
			ob:             OBResult{Score: 1.0},
			wantGatePassed: true,
			wantLabel:      LabelHighConviction,
			wantInvestNil:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateRating(tt.bfs, tt.cds, tt.nfr, tt.pps, tt.vrs, tt.ob)

			if result.GatePassed != tt.wantGatePassed {
				t.Errorf("CalculateRating() gate = %v, want %v", result.GatePassed, tt.wantGatePassed)
			}

			if result.Label != tt.wantLabel {
				t.Errorf("CalculateRating() label = %v, want %v", result.Label, tt.wantLabel)
			}

			if tt.wantInvestNil && result.Investability != nil {
				t.Errorf("CalculateRating() investability should be nil when gate fails")
			}

			if !tt.wantInvestNil && result.Investability == nil {
				t.Errorf("CalculateRating() investability should not be nil when gate passes")
			}

			if result.Reasoning == "" {
				t.Error("CalculateRating() reasoning should not be empty")
			}
		})
	}
}

func TestInvestabilityFormula(t *testing.T) {
	// Test that investability is calculated correctly
	// Formula: BFS*12.5 + CDS*12.5 + NFR*25 + PPS*25 + VRS*15 + OB*10

	bfs := BFSResult{Score: 2}
	cds := CDSResult{Score: 2}
	nfr := NFRResult{Score: 1.0}
	pps := PPSResult{Score: 1.0}
	vrs := VRSResult{Score: 1.0}
	ob := OBResult{Score: 1.0}

	result := CalculateRating(bfs, cds, nfr, pps, vrs, ob)

	if result.Investability == nil {
		t.Fatal("Investability should not be nil")
	}

	// With all max scores:
	// BFS: 2/2 * 12.5 = 12.5
	// CDS: 2/2 * 12.5 = 12.5
	// NFR: 1.0 * 25 = 25
	// PPS: 1.0 * 25 = 25
	// VRS: 1.0 * 15 = 15
	// OB:  1.0 * 10 = 10
	// Total = 100
	expected := 100.0
	actual := *result.Investability

	if actual != expected {
		t.Errorf("Investability = %f, want %f", actual, expected)
	}
}
