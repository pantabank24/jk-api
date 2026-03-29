package entity

import (
	"encoding/json"
	"time"
)

// ParseFormulaSteps decodes a JSON string into a slice of FormulaStep.
func ParseFormulaSteps(raw string) ([]FormulaStep, error) {
	if raw == "" || raw == "null" {
		return nil, nil
	}
	var steps []FormulaStep
	if err := json.Unmarshal([]byte(raw), &steps); err != nil {
		return nil, err
	}
	return steps, nil
}

// PriceSource defines which price field to use as the starting value for calculation.
// bar_buy | bar_sell | ornament_buy | ornament_sell
//
// FormulaSteps is a JSON-encoded array of FormulaStep describing sequential operations
// applied to the price source value to produce the per-gram price.
// e.g. [{"operator":"*","value":0.965},{"operator":"+","value":150}]
// means: perGram = price * 0.965 + 150
type GoldType struct {
	ID             uint      `json:"id"              gorm:"primaryKey"`
	Name           string    `json:"name"            gorm:"type:varchar(100);not null"`
	Description    string    `json:"description"     gorm:"type:text;default:''"`
	PriceSource    string    `json:"price_source"    gorm:"type:varchar(30);default:'bar_buy'"`
	DefaultPercent float64   `json:"default_percent" gorm:"type:decimal(10,4);default:0"`
	DefaultPlus    float64   `json:"default_plus"    gorm:"type:decimal(12,2);default:0"`
	FormulaSteps   string    `json:"formula_steps"   gorm:"type:text;default:'[]'"`
	ServiceRate    float64   `json:"service_rate"    gorm:"type:decimal(15,8);default:0"` // ค่าตัวคูณกำหนดเองต่อประเภท
	PlusType       int       `json:"plus_type"       gorm:"default:0"`                     // 0=บาท, 1=%
	SortOrder      int       `json:"sort_order"      gorm:"default:0"`
	IsActive       bool      `json:"is_active"       gorm:"default:true"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// FormulaStep is a single operation in a gold type's price formula.
//
// OperandType selects which value to use as the right-hand operand:
//   - "number"  → fixed constant stored in Value
//   - "price"   → gold price looked up from price_source at quotation time
//   - "percent" → percentage entered by the user at quotation time
//   - "plus"    → "ราคาบวก" entered by the user at quotation time
//   - "weight"  → weight (grams) entered by the user at quotation time
type FormulaStep struct {
	Operator    string  `json:"operator"`     // +, -, *, /
	OperandType string  `json:"operand_type"` // "number"|"price"|"percent"|"plus"|"weight"
	Value       float64 `json:"value"`        // used only when OperandType == "number"
}

// FormulaVars holds all runtime values available to a formula.
type FormulaVars struct {
	Price   float64 // gold price from price_source
	Percent float64 // purity / percentage entered by user
	Plus    float64 // ราคาบวก entered by user
	Weight  float64 // weight in grams entered by user
}

// ApplyFormula evaluates the formula steps sequentially, starting from the gold
// price (vars.Price) as the initial value.
// Falls back to the legacy DefaultPercent/DefaultPlus formula when FormulaSteps
// is empty or unparseable.
func (gt *GoldType) ApplyFormula(vars FormulaVars) float64 {
	steps, err := ParseFormulaSteps(gt.FormulaSteps)
	if err != nil || len(steps) == 0 {
		return vars.Price*(gt.DefaultPercent/100) + gt.DefaultPlus
	}
	result := vars.Price
	for _, s := range steps {
		var operand float64
		switch s.OperandType {
		case "price":
			operand = vars.Price
		case "percent":
			operand = vars.Percent
		case "plus":
			operand = vars.Plus
		case "weight":
			operand = vars.Weight
		case "service":
			operand = gt.ServiceRate
		default: // "number"
			operand = s.Value
		}
		switch s.Operator {
		case "+":
			result += operand
		case "-":
			result -= operand
		case "*":
			result *= operand
		case "/":
			if operand != 0 {
				result /= operand
			}
		}
	}
	return result
}
