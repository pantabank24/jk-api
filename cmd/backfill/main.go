// Command backfill repairs quotation_items corrupted by the old edit-save bug,
// which wrote the item's own row id into type_id (instead of the gold type id)
// and stored a wrongly-computed per_gram/total (e.g. double-counted weight).
//
// It touches ONLY corrupted rows — those whose type_id does not point to a gold
// type whose name matches the snapshotted type_name — so correct historical
// records keep their stored values. For each corrupted row it remaps type_id
// from type_name (when unambiguous) and recomputes per_gram/total with the real
// pricing engine, then recomputes total_amount for affected quotations.
//
// Run once against an existing database:  make backfill
package main

import (
	"flag"
	"fmt"
	"log"

	"jk-api/config"
	"jk-api/internal/entity"
	"jk-api/pkg/database"
)

func main() {
	apply := flag.Bool("apply", false, "write changes; without it the command only reports (dry run)")
	flag.Parse()

	cfg := config.LoadConfig()
	db, err := database.NewPostgresDB(cfg)
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	if !*apply {
		log.Printf("DRY RUN — no changes will be written (pass -apply to commit)")
	}

	var goldTypes []entity.GoldType
	if err := db.Find(&goldTypes).Error; err != nil {
		log.Fatalf("load gold types: %v", err)
	}
	byID := map[string]entity.GoldType{}
	byName := map[string]entity.GoldType{}
	nameCount := map[string]int{}
	for _, gt := range goldTypes {
		byID[fmt.Sprint(gt.ID)] = gt
		byName[gt.Name] = gt
		nameCount[gt.Name]++
	}

	var items []entity.QuotationItem
	if err := db.Find(&items).Error; err != nil {
		log.Fatalf("load quotation items: %v", err)
	}

	fixedItems, skipped := 0, 0
	affected := map[uint]bool{}

	for _, it := range items {
		// Healthy: type_id points to a gold type whose name matches type_name.
		if gt, ok := byID[it.TypeID]; ok && gt.Name == it.TypeName {
			continue
		}
		// Corrupted — repair only when the name maps to exactly one gold type.
		if nameCount[it.TypeName] != 1 {
			skipped++
			log.Printf("skip item %d: type_name %q maps to %d gold types", it.ID, it.TypeName, nameCount[it.TypeName])
			continue
		}
		gt := byName[it.TypeName]
		perGram, total := gt.ComputeItem(it.Price, it.Percent, it.Plus, it.Weight)
		log.Printf("item %d (quotation %d): type_id %q→%d, total %.2f→%.2f",
			it.ID, it.QuotationID, it.TypeID, gt.ID, it.Total, total)
		if *apply {
			if err := db.Model(&entity.QuotationItem{}).Where("id = ?", it.ID).
				Updates(map[string]interface{}{
					"type_id":  fmt.Sprint(gt.ID),
					"per_gram": perGram,
					"total":    total,
				}).Error; err != nil {
				log.Printf("update item %d: %v", it.ID, err)
				continue
			}
		}
		fixedItems++
		affected[it.QuotationID] = true
	}

	// Recompute total_amount from the (now-correct) item totals for each quotation
	// that had a repaired item.
	fixedQuotations := 0
	if *apply {
		for qid := range affected {
			var sum float64
			db.Model(&entity.QuotationItem{}).Where("quotation_id = ?", qid).
				Select("COALESCE(SUM(total), 0)").Scan(&sum)
			if err := db.Model(&entity.Quotation{}).Where("id = ?", qid).
				Update("total_amount", sum).Error; err != nil {
				log.Printf("update quotation %d total: %v", qid, err)
				continue
			}
			fixedQuotations++
		}
	} else {
		fixedQuotations = len(affected)
	}

	mode := "DRY RUN — would fix"
	if *apply {
		mode = "✅ fixed"
	}
	log.Printf("%s: items=%d, quotations recalculated=%d, items skipped(ambiguous name)=%d",
		mode, fixedItems, fixedQuotations, skipped)
}
