package cli

import (
	"context"
	"fmt"
	"sort"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/overplane/overplane/internal/container"
	"github.com/overplane/overplane/internal/platform/color"
	oplog "github.com/overplane/overplane/internal/platform/log"
	"github.com/overplane/overplane/internal/platform/serde/canonjson"
	"github.com/overplane/overplane/internal/platform/timeutil"
	"github.com/overplane/overplane/internal/recipes"
)

func agentListImagesHelp() color.HelpSpec {
	return color.HelpSpec{
		Command: "agent list-images",
		Usage:   Binary + " agent list-images [--json]",
		Description: "List this project's agent container images: every local image labeled " +
			"overplane.project=<project.name>, with tag, id, size, creation time, setup recipe, " +
			"base image, and build hash. Zero images is success with a hint to run 'agent setup'.",
		Flags: []color.HelpFlag{
			{Name: "--json", Description: "emit the records as a canonical JSON array"},
		},
		Examples: []string{Binary + " agent list-images", Binary + " agent list-images --json"},
	}
}

// agentImageRecord is one list-images row; field order matches the table.
type agentImageRecord struct {
	Tag       string `json:"tag"`
	ImageID   string `json:"image_id"`
	SizeBytes int64  `json:"size_bytes"`
	Created   string `json:"created"`
	Recipe    string `json:"recipe"`
	BaseImage string `json:"base_image"`
	BuildHash string `json:"build_hash"`
}

func (c agentCommand) listImages(ctx context.Context, args []string) error {
	if len(args) > 0 && isHelpToken(args[0]) {
		fmt.Fprint(c.r.Out, usage(agentListImagesHelp()))
		return nil
	}
	asJSON, rest := wantsJSON(args)
	if len(rest) > 0 {
		return UsageError("agent list-images takes no arguments besides --json")
	}
	cfg, _, err := requireProject(c.r)
	if err != nil {
		return err
	}
	client, err := agentEngineClient(ctx, cfg.Agent.Container.Runtime)
	if err != nil {
		return err
	}
	images, err := client.ListLocalImages(ctx, container.ImageFilter{
		Labels: map[string]string{recipes.LabelProject: cfg.Project.Name},
	})
	if err != nil {
		return withHint(err)
	}
	records := agentImageRecords(images, recipes.LatestTag(cfg.Project.Name))
	oplog.FromContext(ctx).Info("listed agent images",
		"step", "agent-list-images", "project", cfg.Project.Name, "count", len(records))
	if asJSON {
		return c.printImagesJSON(records)
	}
	c.printImagesTable(records)
	return nil
}

// agentImageRecords converts engine rows to output records, :latest first,
// then by tag.
func agentImageRecords(images []container.Image, latestTag string) []agentImageRecord {
	records := make([]agentImageRecord, 0, len(images))
	for _, img := range images {
		created := ""
		if !img.Created.IsZero() {
			created = timeutil.Stamp(img.Created)
		}
		records = append(records, agentImageRecord{
			Tag:       img.Ref.String(),
			ImageID:   shortImageID(img.ID),
			SizeBytes: img.Size,
			Created:   created,
			Recipe:    img.Labels[recipes.LabelBuildRecipe],
			BaseImage: img.Labels[recipes.LabelBuildBase],
			BuildHash: img.Labels[recipes.LabelBuildHash],
		})
	}
	sort.SliceStable(records, func(i, j int) bool {
		if (records[i].Tag == latestTag) != (records[j].Tag == latestTag) {
			return records[i].Tag == latestTag
		}
		return records[i].Tag < records[j].Tag
	})
	return records
}

func (c agentCommand) printImagesJSON(records []agentImageRecord) error {
	b, err := canonjson.MarshalIndent(records, "", "  ")
	if err != nil {
		return InternalError(err)
	}
	fmt.Fprintln(c.r.Out, string(b))
	return nil
}

func (c agentCommand) printImagesTable(records []agentImageRecord) {
	if len(records) == 0 {
		fmt.Fprintf(c.r.Out, "no agent images for this project; run %s first\n",
			color.Sprint(4, Binary+" agent setup"))
		return
	}
	t := color.Table(c.r.Out)
	t.AppendHeader(table.Row{"Tag", "Image ID", "Size", "Created", "Recipe", "Base", "Build hash"})
	for _, r := range records {
		t.AppendRow(table.Row{
			color.Sprint(4, r.Tag), r.ImageID, formatImageSize(r.SizeBytes),
			r.Created, r.Recipe, r.BaseImage, r.BuildHash,
		})
	}
	t.Render()
}
