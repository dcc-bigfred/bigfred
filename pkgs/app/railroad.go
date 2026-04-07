package app

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// locoColumns is the ordered list of columns in the loco table (excluding _id).
var locoColumns = []string{
	"image", "text", "description", "address", "type",
	"speed_pos", "func_en", "func_sts", "func_type_arr", "func_pos",
	"inverted", "mapping", "logon_id", "ordercol", "loco_group", "source",
}

// RailroadCpArgs holds parameters for the RailroadCp action.
type RailroadCpArgs struct {
	SrcFile  string
	DstFile  string
	LocoName string
}

// RailroadCp copies a loco row identified by name from SrcFile database to DstFile database.
func (app *LocoApp) RailroadCp(args RailroadCpArgs) error {
	srcDB, err := sql.Open("sqlite", args.SrcFile)
	if err != nil {
		return fmt.Errorf("cannot open source database %q: %w", args.SrcFile, err)
	}
	defer srcDB.Close()

	dstDB, err := sql.Open("sqlite", args.DstFile)
	if err != nil {
		return fmt.Errorf("cannot open destination database %q: %w", args.DstFile, err)
	}
	defer dstDB.Close()

	// Build SELECT query for all non-PK columns
	selectSQL := "SELECT image, text, description, address, type, speed_pos, func_en, func_sts, func_type_arr, func_pos, inverted, mapping, logon_id, ordercol, loco_group, source FROM loco WHERE text = ?"
	row := srcDB.QueryRow(selectSQL, args.LocoName)

	vals := make([]any, len(locoColumns))
	ptrs := make([]any, len(locoColumns))
	for i := range vals {
		ptrs[i] = &vals[i]
	}

	if err := row.Scan(ptrs...); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("loco %q not found in %s", args.LocoName, args.SrcFile)
		}
		return fmt.Errorf("error reading loco %q: %w", args.LocoName, err)
	}

	// Check if loco already exists in destination
	var existingID int64
	checkErr := dstDB.QueryRow("SELECT _id FROM loco WHERE text = ?", args.LocoName).Scan(&existingID)
	if checkErr != nil && checkErr != sql.ErrNoRows {
		return fmt.Errorf("error checking destination database: %w", checkErr)
	}

	if checkErr == nil {
		// UPDATE existing row
		updateSQL := buildUpdateSQL(locoColumns)
		updateArgs := append(vals, existingID)
		if _, err := dstDB.Exec(updateSQL, updateArgs...); err != nil {
			return fmt.Errorf("error updating loco %q in destination: %w", args.LocoName, err)
		}
		app.P.Printf("Updated loco %q (id=%d) in %s\n", args.LocoName, existingID, args.DstFile)
	} else {
		// INSERT new row
		insertSQL := buildInsertSQL(locoColumns)
		if _, err := dstDB.Exec(insertSQL, vals...); err != nil {
			return fmt.Errorf("error inserting loco %q into destination: %w", args.LocoName, err)
		}
		app.P.Printf("Copied loco %q to %s\n", args.LocoName, args.DstFile)
	}

	return nil
}

func buildInsertSQL(cols []string) string {
	colList := ""
	placeholders := ""
	for i, c := range cols {
		if i > 0 {
			colList += ", "
			placeholders += ", "
		}
		colList += c
		placeholders += "?"
	}
	return fmt.Sprintf("INSERT INTO loco (%s) VALUES (%s)", colList, placeholders)
}

func buildUpdateSQL(cols []string) string {
	sets := ""
	for i, c := range cols {
		if i > 0 {
			sets += ", "
		}
		sets += c + " = ?"
	}
	return fmt.Sprintf("UPDATE loco SET %s WHERE _id = ?", sets)
}
