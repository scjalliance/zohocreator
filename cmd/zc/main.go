// zc is a CLI for exercising the zohocreator Go client. It reads API
// credentials from environment variables and exposes every major Creator v2.1
// operation through flag-oriented subcommands.
//
// Typical usage (fish shell + envwith, per project standards):
//
//	envwith -f .secrets/.env -- go run ./cmd/zc whoami
//	envwith -f .secrets/.env -- go run ./cmd/zc apps
//	envwith -f .secrets/.env -- go run ./cmd/zc records owner app All_Orders -all -max 1000
//	envwith -f .secrets/.env -- go run ./cmd/zc record owner app All_Orders 3888833000000114027
//	envwith -f .secrets/.env -- go run ./cmd/zc add owner app My_Form -file ./records.json
//	envwith -f .secrets/.env -- go run ./cmd/zc bulk-create owner app All_Orders -max 100000
//	envwith -f .secrets/.env -- go run ./cmd/zc bulk-result owner app All_Orders <jobid> -out out.zip
//	envwith -f .secrets/.env -- go run ./cmd/zc custom jason my-custom-api -method POST -body body.json
//
// Environment variables:
//
//	ZOHO_CLIENT_ID       OAuth client ID from Zoho API console
//	ZOHO_CLIENT_SECRET   OAuth client secret
//	ZOHO_REFRESH_TOKEN   Long-lived refresh token (from `zc oauth exchange`)
//	ZOHO_ACCESS_TOKEN    Optional: seed initial access token (still refreshes)
//	ZOHO_DATA_CENTER     us | eu | in | au | jp | ca | cn | sa | ae (default us)
//	ZOHO_ENV             production | stage | development (default production)
//
// The oauth subcommands work without ZOHO_REFRESH_TOKEN, so this CLI can be
// used to bootstrap a fresh OAuth setup.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/scjalliance/zohocreator"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	ctx := context.Background()

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "oauth":
		oauthCmd(ctx, args)
	case "whoami":
		runWithClient(ctx, args, whoamiCmd)
	case "apps":
		runWithClient(ctx, args, appsCmd)
	case "apps-by":
		runWithClient(ctx, args, appsByCmd)
	case "forms":
		runWithClient(ctx, args, formsCmd)
	case "reports":
		runWithClient(ctx, args, reportsCmd)
	case "fields":
		runWithClient(ctx, args, fieldsCmd)
	case "pages":
		runWithClient(ctx, args, pagesCmd)
	case "sections":
		runWithClient(ctx, args, sectionsCmd)
	case "records":
		runWithClient(ctx, args, recordsCmd)
	case "record":
		runWithClient(ctx, args, recordCmd)
	case "add":
		runWithClient(ctx, args, addCmd)
	case "update":
		runWithClient(ctx, args, updateCmd)
	case "delete":
		runWithClient(ctx, args, deleteCmd)
	case "download":
		runWithClient(ctx, args, downloadCmd)
	case "upload":
		runWithClient(ctx, args, uploadCmd)
	case "bulk-create":
		runWithClient(ctx, args, bulkCreateCmd)
	case "bulk-status":
		runWithClient(ctx, args, bulkStatusCmd)
	case "bulk-result":
		runWithClient(ctx, args, bulkResultCmd)
	case "custom":
		runWithClient(ctx, args, customCmd)
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", cmd)
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `zc — Zoho Creator CLI

Auth (no access required):
  oauth authurl -client <id> [-dc us] [-redirect URI] [-scopes ...]   Print consent URL
  oauth exchange <code> -client <id> -secret <sec> -redirect URI     Trade code for tokens
  oauth refresh                                                       Print a fresh access token

Read-only:
  whoami                                             Probe credentials, list apps count
  apps                                               List all accessible apps
  apps-by <owner>                                    List apps owned by a workspace
  forms <owner> <app>                                List forms
  reports <owner> <app>                              List reports
  fields <owner> <app> <form>                        List fields
  pages <owner> <app>                                List pages
  sections <owner> <app>                             List nav sections
  records <owner> <app> <report> [flags]             List/stream records
  record <owner> <app> <report> <id>                 Get one record (detail view)
  download <owner> <app> <report> <id> <field> -out FILE

Mutating:
  add <owner> <app> <form> -file JSONFILE            Add records (JSON array of maps)
  update <owner> <app> <report> <id> -file JSONFILE  Patch record
  delete <owner> <app> <report> <id>                 Delete record
  upload <owner> <app> <report> <id> <field> -file LOCAL

Bulk:
  bulk-create <owner> <app> <report> [-criteria EXPR -fields a,b -max N]
  bulk-status <owner> <app> <report> <jobid>
  bulk-result <owner> <app> <report> <jobid> -out FILE

Custom APIs:
  custom <appadmin> <apiname> [-method GET -body FILE -query k=v,l=w -publickey KEY]

Environment:
  ZOHO_CLIENT_ID, ZOHO_CLIENT_SECRET, ZOHO_REFRESH_TOKEN, ZOHO_ACCESS_TOKEN,
  ZOHO_DATA_CENTER (us|eu|in|au|jp|ca|cn|sa|ae), ZOHO_ENV (production|stage|development),
  ZOHO_API_VERSION (v2.1 default; v2 for legacy Creator 5 tenants)`)
}

func runWithClient(ctx context.Context, args []string, fn func(context.Context, *zohocreator.Client, []string)) {
	c, err := buildClient()
	if err != nil {
		fatal("build client: %v", err)
	}
	fn(ctx, c, args)
}

func buildClient() (*zohocreator.Client, error) {
	dcRaw := envOr("ZOHO_DATA_CENTER", "us")
	dc, err := zohocreator.ParseDataCenter(dcRaw)
	if err != nil {
		return nil, err
	}
	cfg := zohocreator.Config{
		DataCenter:   dc,
		ClientID:     os.Getenv("ZOHO_CLIENT_ID"),
		ClientSecret: os.Getenv("ZOHO_CLIENT_SECRET"),
		RefreshToken: os.Getenv("ZOHO_REFRESH_TOKEN"),
		AccessToken:  os.Getenv("ZOHO_ACCESS_TOKEN"),
		Environment:  zohocreator.Environment(envOr("ZOHO_ENV", string(zohocreator.EnvProduction))),
		APIVersion:   zohocreator.APIVersion(envOr("ZOHO_API_VERSION", string(zohocreator.APIVersionV21))),
	}
	return zohocreator.NewClient(cfg)
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

// --- oauth subcommand ---------------------------------------------------

func oauthCmd(ctx context.Context, args []string) {
	if len(args) == 0 {
		fatal("oauth: expected subcommand (authurl|exchange|refresh)")
	}
	sub := args[0]
	rest := args[1:]
	switch sub {
	case "authurl":
		oauthAuthURL(rest)
	case "exchange":
		oauthExchange(ctx, rest)
	case "refresh":
		oauthRefresh(ctx)
	default:
		fatal("oauth: unknown subcommand %q", sub)
	}
}

func oauthAuthURL(args []string) {
	fs := flag.NewFlagSet("oauth authurl", flag.ExitOnError)
	client := fs.String("client", os.Getenv("ZOHO_CLIENT_ID"), "OAuth client ID")
	dc := fs.String("dc", envOr("ZOHO_DATA_CENTER", "us"), "data center")
	redirect := fs.String("redirect", "http://localhost:8080/callback", "redirect URI registered in API console")
	scopes := fs.String("scopes", "ZohoCreator.dashboard.READ,ZohoCreator.meta.application.READ,ZohoCreator.meta.form.READ,ZohoCreator.report.READ,ZohoCreator.form.CREATE,ZohoCreator.report.UPDATE,ZohoCreator.report.DELETE,ZohoCreator.report.CREATE,ZohoCreator.bulk.READ,ZohoCreator.bulk.CREATE", "comma-separated scope list")
	state := fs.String("state", "zc-cli", "state parameter")
	_ = fs.Parse(args)
	if *client == "" {
		fatal("oauth authurl: -client is required")
	}
	parsed, err := zohocreator.ParseDataCenter(*dc)
	if err != nil {
		fatal("%v", err)
	}
	u := zohocreator.AuthURL(parsed, *client, *redirect, strings.Split(*scopes, ","), *state)
	fmt.Println(u)
}

func oauthExchange(ctx context.Context, args []string) {
	fs := flag.NewFlagSet("oauth exchange", flag.ExitOnError)
	client := fs.String("client", os.Getenv("ZOHO_CLIENT_ID"), "OAuth client ID")
	secret := fs.String("secret", os.Getenv("ZOHO_CLIENT_SECRET"), "OAuth client secret")
	dc := fs.String("dc", envOr("ZOHO_DATA_CENTER", "us"), "data center")
	redirect := fs.String("redirect", "http://localhost:8080/callback", "redirect URI (must match AuthURL)")
	_ = fs.Parse(args)
	if fs.NArg() == 0 {
		fatal("oauth exchange: authorization code positional arg required")
	}
	code := fs.Arg(0)
	parsed, err := zohocreator.ParseDataCenter(*dc)
	if err != nil {
		fatal("%v", err)
	}
	res, err := zohocreator.ExchangeCode(ctx, parsed, *client, *secret, code, *redirect, nil)
	if err != nil {
		fatal("exchange: %v", err)
	}
	fmt.Printf("access_token:  %s\nrefresh_token: %s\nexpires_in:    %ds\napi_domain:    %s\n",
		res.AccessToken, res.RefreshToken, res.ExpiresIn, res.APIDomain)
}

func oauthRefresh(ctx context.Context) {
	c, err := buildClient()
	if err != nil {
		fatal("build client: %v", err)
	}
	tok, err := c.TokenSource().Token(ctx)
	if err != nil {
		fatal("refresh: %v", err)
	}
	fmt.Println(tok)
}

// --- read-only commands -------------------------------------------------

func whoamiCmd(ctx context.Context, c *zohocreator.Client, _ []string) {
	apps, err := c.Meta.Applications(ctx)
	if err != nil {
		fatal("whoami: %v", err)
	}
	fmt.Printf("OK — base URL: %s\n", c.BaseURL())
	fmt.Printf("Applications visible: %d\n", len(apps))
}

func appsCmd(ctx context.Context, c *zohocreator.Client, _ []string) {
	apps, err := c.Meta.Applications(ctx)
	if err != nil {
		fatal("apps: %v", err)
	}
	tw := newTab()
	fmt.Fprintln(tw, "WORKSPACE\tLINK_NAME\tAPPLICATION_NAME\tCATEGORY\tTIMEZONE")
	for _, a := range apps {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%d\t%s\n",
			a.WorkspaceName, a.LinkName, a.ApplicationName, a.Category, a.TimeZone)
	}
	_ = tw.Flush()
}

func appsByCmd(ctx context.Context, c *zohocreator.Client, args []string) {
	if len(args) < 1 {
		fatal("apps-by: <owner> required")
	}
	apps, err := c.Meta.ApplicationsByWorkspace(ctx, args[0])
	if err != nil {
		fatal("apps-by: %v", err)
	}
	printJSON(apps)
}

func formsCmd(ctx context.Context, c *zohocreator.Client, args []string) {
	if len(args) < 2 {
		fatal("forms: <owner> <app> required")
	}
	forms, err := c.Meta.Forms(ctx, args[0], args[1])
	if err != nil {
		fatal("forms: %v", err)
	}
	tw := newTab()
	fmt.Fprintln(tw, "LINK_NAME\tDISPLAY_NAME\tTYPE")
	for _, f := range forms {
		fmt.Fprintf(tw, "%s\t%s\t%d\n", f.LinkName, f.DisplayName, f.Type)
	}
	_ = tw.Flush()
}

func reportsCmd(ctx context.Context, c *zohocreator.Client, args []string) {
	if len(args) < 2 {
		fatal("reports: <owner> <app> required")
	}
	rs, err := c.Meta.Reports(ctx, args[0], args[1])
	if err != nil {
		fatal("reports: %v", err)
	}
	tw := newTab()
	fmt.Fprintln(tw, "LINK_NAME\tDISPLAY_NAME\tTYPE")
	for _, r := range rs {
		fmt.Fprintf(tw, "%s\t%s\t%d\n", r.LinkName, r.DisplayName, r.Type)
	}
	_ = tw.Flush()
}

func fieldsCmd(ctx context.Context, c *zohocreator.Client, args []string) {
	if len(args) < 3 {
		fatal("fields: <owner> <app> <form> required")
	}
	fs, err := c.Meta.Fields(ctx, args[0], args[1], args[2])
	if err != nil {
		fatal("fields: %v", err)
	}
	tw := newTab()
	fmt.Fprintln(tw, "LINK_NAME\tDISPLAY_NAME\tTYPE\tMANDATORY\tUNIQUE\tMAX_CHAR")
	for _, f := range fs {
		fmt.Fprintf(tw, "%s\t%s\t%d (%s)\t%v\t%v\t%d\n",
			f.LinkName, f.DisplayName, f.Type, zohocreator.FieldTypeName(f.Type),
			f.Mandatory, f.Unique, f.MaxChar)
	}
	_ = tw.Flush()
}

func pagesCmd(ctx context.Context, c *zohocreator.Client, args []string) {
	if len(args) < 2 {
		fatal("pages: <owner> <app> required")
	}
	ps, err := c.Meta.Pages(ctx, args[0], args[1])
	if err != nil {
		fatal("pages: %v", err)
	}
	tw := newTab()
	fmt.Fprintln(tw, "LINK_NAME\tDISPLAY_NAME")
	for _, p := range ps {
		fmt.Fprintf(tw, "%s\t%s\n", p.LinkName, p.DisplayName)
	}
	_ = tw.Flush()
}

func sectionsCmd(ctx context.Context, c *zohocreator.Client, args []string) {
	if len(args) < 2 {
		fatal("sections: <owner> <app> required")
	}
	s, err := c.Meta.Sections(ctx, args[0], args[1])
	if err != nil {
		fatal("sections: %v", err)
	}
	printJSON(s)
}

func recordsCmd(ctx context.Context, c *zohocreator.Client, args []string) {
	if len(args) < 3 {
		fatal("records: <owner> <app> <report> required")
	}
	owner, app, rpt := args[0], args[1], args[2]
	fs := flag.NewFlagSet("records", flag.ExitOnError)
	criteria := fs.String("criteria", "", "Creator criteria expression")
	cursor := fs.String("cursor", "", "record_cursor from a prior page")
	maxRecords := fs.Int("max", 0, "max_records (200/500/1000; default server)")
	limit := fs.Int("limit", 0, "limit parameter")
	from := fs.Int("from", 0, "from offset")
	fieldsList := fs.String("fields", "", "comma-separated field link names (implies -field-config=custom)")
	fieldConfig := fs.String("field-config", "", "quick_view|detail_view|custom|all")
	all := fs.Bool("all", false, "stream across all pages via cursor")
	outFmt := fs.String("format", "json", "output format: json|ndjson|csv")
	_ = fs.Parse(args[3:])

	q := zohocreator.NewQuery()
	if *criteria != "" {
		q.CriteriaExpr(*criteria)
	}
	if *cursor != "" {
		q.Cursor(*cursor)
	}
	if *maxRecords > 0 {
		q.MaxRecordsN(*maxRecords)
	}
	if *limit > 0 {
		q.LimitN(*limit)
	}
	if *from > 0 {
		q.FromOffset(*from)
	}
	if *fieldsList != "" {
		q.FieldsList(strings.Split(*fieldsList, ",")...)
		if *fieldConfig == "" {
			*fieldConfig = string(zohocreator.FieldConfigCustom)
		}
	}
	if *fieldConfig != "" {
		q.FieldConfigMode(zohocreator.FieldConfig(*fieldConfig))
	}

	if *all {
		seq := c.Records.All(ctx, owner, app, rpt, q)
		emitRecords(seq, *outFmt)
		return
	}
	page, err := c.Records.Get(ctx, owner, app, rpt, q)
	if err != nil {
		fatal("records: %v", err)
	}
	if page.HasNext() {
		if page.Cursor != "" {
			fmt.Fprintf(os.Stderr, "(next cursor: %s — pass -cursor or use -all)\n", page.Cursor)
		} else {
			nextFrom := *from + len(page.Items)
			if *limit > 0 {
				nextFrom = *from + *limit
			}
			fmt.Fprintf(os.Stderr, "(more records available — pass -from %d or use -all)\n", nextFrom)
		}
	}
	emitSlice(page.Items, *outFmt)
}

func emitRecords(seq func(yield func(zohocreator.Record, error) bool), format string) {
	switch format {
	case "ndjson":
		enc := json.NewEncoder(os.Stdout)
		for rec, err := range seq {
			if err != nil {
				fatal("iter: %v", err)
			}
			_ = enc.Encode(rec)
		}
	case "csv":
		rows := []zohocreator.Record{}
		for rec, err := range seq {
			if err != nil {
				fatal("iter: %v", err)
			}
			rows = append(rows, rec)
		}
		writeCSV(rows)
	default:
		rows := []zohocreator.Record{}
		for rec, err := range seq {
			if err != nil {
				fatal("iter: %v", err)
			}
			rows = append(rows, rec)
		}
		printJSON(rows)
	}
}

func emitSlice(items []zohocreator.Record, format string) {
	switch format {
	case "ndjson":
		enc := json.NewEncoder(os.Stdout)
		for _, it := range items {
			_ = enc.Encode(it)
		}
	case "csv":
		writeCSV(items)
	default:
		printJSON(items)
	}
}

func writeCSV(rows []zohocreator.Record) {
	if len(rows) == 0 {
		return
	}
	fields := []string{}
	seen := map[string]struct{}{}
	for _, r := range rows {
		for k := range r {
			if _, ok := seen[k]; !ok {
				seen[k] = struct{}{}
				fields = append(fields, k)
			}
		}
	}
	sort.Strings(fields)
	fmt.Println(strings.Join(fields, ","))
	for _, r := range rows {
		cells := make([]string, len(fields))
		for i, f := range fields {
			cells[i] = csvEscape(r.String(f))
		}
		fmt.Println(strings.Join(cells, ","))
	}
}

func csvEscape(s string) string {
	if !strings.ContainsAny(s, `",`+"\r\n") {
		return s
	}
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

func recordCmd(ctx context.Context, c *zohocreator.Client, args []string) {
	if len(args) < 4 {
		fatal("record: <owner> <app> <report> <id> required")
	}
	rec, err := c.Records.GetByID(ctx, args[0], args[1], args[2], args[3])
	if err != nil {
		fatal("record: %v", err)
	}
	printJSON(rec)
}

func addCmd(ctx context.Context, c *zohocreator.Client, args []string) {
	if len(args) < 3 {
		fatal("add: <owner> <app> <form> required")
	}
	owner, app, form := args[0], args[1], args[2]
	fs := flag.NewFlagSet("add", flag.ExitOnError)
	file := fs.String("file", "", "path to a JSON file containing an array of record objects")
	message := fs.Bool("message", false, "return workflow messages")
	tasks := fs.Bool("tasks", false, "return workflow tasks")
	returnFields := fs.String("fields", "", "comma-separated field link names to echo back")
	skip := fs.String("skip-workflow", "", "comma-separated workflow triggers to skip")
	_ = fs.Parse(args[3:])
	if *file == "" {
		fatal("add: -file required")
	}
	records := loadRecords(*file)
	var opts *zohocreator.AddOptions
	if *message || *tasks || *returnFields != "" || *skip != "" {
		opts = &zohocreator.AddOptions{Message: *message, Tasks: *tasks}
		if *returnFields != "" {
			opts.ReturnFields = strings.Split(*returnFields, ",")
		}
		if *skip != "" {
			opts.SkipWorkflow = strings.Split(*skip, ",")
		}
	}
	res, err := c.Records.Add(ctx, owner, app, form, records, opts)
	if err != nil {
		fatal("add: %v", err)
	}
	printJSON(res)
}

func updateCmd(ctx context.Context, c *zohocreator.Client, args []string) {
	if len(args) < 4 {
		fatal("update: <owner> <app> <report> <id> required")
	}
	owner, app, rpt, id := args[0], args[1], args[2], args[3]
	fs := flag.NewFlagSet("update", flag.ExitOnError)
	file := fs.String("file", "", "path to JSON object with field updates")
	_ = fs.Parse(args[4:])
	if *file == "" {
		fatal("update: -file required")
	}
	rec := loadRecord(*file)
	res, err := c.Records.UpdateByID(ctx, owner, app, rpt, id, rec, nil)
	if err != nil {
		fatal("update: %v", err)
	}
	printJSON(res)
}

func deleteCmd(ctx context.Context, c *zohocreator.Client, args []string) {
	if len(args) < 4 {
		fatal("delete: <owner> <app> <report> <id> required")
	}
	res, err := c.Records.DeleteByID(ctx, args[0], args[1], args[2], args[3], nil)
	if err != nil {
		fatal("delete: %v", err)
	}
	printJSON(res)
}

func downloadCmd(ctx context.Context, c *zohocreator.Client, args []string) {
	if len(args) < 5 {
		fatal("download: <owner> <app> <report> <id> <field> required")
	}
	fs := flag.NewFlagSet("download", flag.ExitOnError)
	out := fs.String("out", "", "output file (default: use server-supplied filename)")
	privatelink := fs.String("privatelink", "", "privatelink query value (skips OAuth)")
	_ = fs.Parse(args[5:])
	owner, app, rpt, id, field := args[0], args[1], args[2], args[3], args[4]

	var (
		dst     io.Writer
		closer  *os.File
		outPath = *out
	)
	if outPath == "" {
		dst = os.Stdout
	} else {
		f, err := os.Create(outPath)
		if err != nil {
			fatal("create %s: %v", outPath, err)
		}
		closer = f
		dst = f
	}
	fn, n, err := c.Files.Download(ctx, owner, app, rpt, id, field, *privatelink, dst)
	if closer != nil {
		if cerr := closer.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}
	if err != nil {
		fatal("download: %v", err)
	}
	fmt.Fprintf(os.Stderr, "%d bytes (server filename: %q)\n", n, fn)
}

func uploadCmd(ctx context.Context, c *zohocreator.Client, args []string) {
	if len(args) < 5 {
		fatal("upload: <owner> <app> <report> <id> <field> required")
	}
	fs := flag.NewFlagSet("upload", flag.ExitOnError)
	file := fs.String("file", "", "local file path to upload")
	ct := fs.String("content-type", "", "override Content-Type")
	_ = fs.Parse(args[5:])
	if *file == "" {
		fatal("upload: -file required")
	}
	f, err := os.Open(*file)
	if err != nil {
		fatal("open %s: %v", *file, err)
	}
	defer func() { _ = f.Close() }()
	name := *file
	if idx := strings.LastIndexAny(name, `/\\`); idx >= 0 {
		name = name[idx+1:]
	}
	owner, app, rpt, id, field := args[0], args[1], args[2], args[3], args[4]
	res, err := c.Files.Upload(ctx, owner, app, rpt, id, field, name, *ct, f)
	if err != nil {
		fatal("upload: %v", err)
	}
	printJSON(res)
}

func bulkCreateCmd(ctx context.Context, c *zohocreator.Client, args []string) {
	if len(args) < 3 {
		fatal("bulk-create: <owner> <app> <report> required")
	}
	fs := flag.NewFlagSet("bulk-create", flag.ExitOnError)
	criteria := fs.String("criteria", "", "criteria expression")
	fieldsList := fs.String("fields", "", "comma-separated field link names")
	maxRecords := fs.Int("max", 0, "max_records (100000..200000)")
	_ = fs.Parse(args[3:])
	q := &zohocreator.BulkReadQuery{
		Criteria:   *criteria,
		MaxRecords: *maxRecords,
	}
	if *fieldsList != "" {
		q.Fields = strings.Split(*fieldsList, ",")
	}
	job, err := c.Bulk.Create(ctx, args[0], args[1], args[2], q)
	if err != nil {
		fatal("bulk-create: %v", err)
	}
	printJSON(job)
}

func bulkStatusCmd(ctx context.Context, c *zohocreator.Client, args []string) {
	if len(args) < 4 {
		fatal("bulk-status: <owner> <app> <report> <jobid> required")
	}
	job, err := c.Bulk.Status(ctx, args[0], args[1], args[2], args[3])
	if err != nil {
		fatal("bulk-status: %v", err)
	}
	printJSON(job)
}

func bulkResultCmd(ctx context.Context, c *zohocreator.Client, args []string) {
	if len(args) < 4 {
		fatal("bulk-result: <owner> <app> <report> <jobid> required")
	}
	fs := flag.NewFlagSet("bulk-result", flag.ExitOnError)
	out := fs.String("out", "", "output path (required)")
	_ = fs.Parse(args[4:])
	if *out == "" {
		fatal("bulk-result: -out required")
	}
	f, err := os.Create(*out)
	if err != nil {
		fatal("create %s: %v", *out, err)
	}
	defer func() { _ = f.Close() }()
	n, err := c.Bulk.DownloadResult(ctx, args[0], args[1], args[2], args[3], f)
	if err != nil {
		fatal("bulk-result: %v", err)
	}
	fmt.Fprintf(os.Stderr, "%d bytes written to %s\n", n, *out)
}

func customCmd(ctx context.Context, c *zohocreator.Client, args []string) {
	if len(args) < 2 {
		fatal("custom: <appadmin> <apiname> required")
	}
	fs := flag.NewFlagSet("custom", flag.ExitOnError)
	method := fs.String("method", "GET", "HTTP method")
	body := fs.String("body", "", "path to body file (JSON unless -raw)")
	raw := fs.Bool("raw", false, "send body verbatim with -content-type")
	contentType := fs.String("content-type", "application/json", "content-type for -raw")
	queryS := fs.String("query", "", "comma-separated k=v query params")
	publickey := fs.String("publickey", "", "public key (bypasses OAuth)")
	_ = fs.Parse(args[2:])

	opts := &zohocreator.CustomAPIOptions{
		Method:    strings.ToUpper(*method),
		PublicKey: *publickey,
	}
	if *body != "" {
		b, err := os.ReadFile(*body)
		if err != nil {
			fatal("read body: %v", err)
		}
		if *raw {
			opts.RawBody = strings.NewReader(string(b))
			opts.ContentType = *contentType
		} else {
			var v any
			if err := json.Unmarshal(b, &v); err != nil {
				fatal("parse body JSON: %v", err)
			}
			opts.Body = v
		}
	}
	if *queryS != "" {
		opts.Query = parseKV(*queryS)
	}
	data, headers, err := c.CustomAPIs.Invoke(ctx, args[0], args[1], opts)
	if err != nil {
		fatal("custom: %v", err)
	}
	fmt.Fprintf(os.Stderr, "Content-Type: %s\n", headers.Get("Content-Type"))
	os.Stdout.Write(data)
	if len(data) > 0 && data[len(data)-1] != '\n' {
		fmt.Println()
	}
}

// --- helpers ------------------------------------------------------------

func loadRecords(path string) []zohocreator.Record {
	b, err := os.ReadFile(path)
	if err != nil {
		fatal("read %s: %v", path, err)
	}
	var recs []zohocreator.Record
	if err := json.Unmarshal(b, &recs); err != nil {
		// tolerate single-object form
		var one zohocreator.Record
		if err2 := json.Unmarshal(b, &one); err2 == nil {
			return []zohocreator.Record{one}
		}
		fatal("parse JSON in %s: %v", path, err)
	}
	return recs
}

func loadRecord(path string) zohocreator.Record {
	b, err := os.ReadFile(path)
	if err != nil {
		fatal("read %s: %v", path, err)
	}
	var r zohocreator.Record
	if err := json.Unmarshal(b, &r); err != nil {
		fatal("parse JSON: %v", err)
	}
	return r
}

func parseKV(s string) map[string][]string {
	out := map[string][]string{}
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		k, v, ok := strings.Cut(p, "=")
		if !ok {
			continue
		}
		out[k] = append(out[k], v)
	}
	return out
}

func printJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func newTab() *tabwriter.Writer {
	return tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
}
