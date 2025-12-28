package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		collection := core.NewBaseCollection("habits")

		collection.Fields.Add(&core.TextField{
			Name:     "name",
			Required: true,
			Min:      1,
			Max:      255,
		})

		collection.Fields.Add(&core.TextField{
			Name:     "description",
			Required: false,
		})

		collection.Fields.Add(&core.SelectField{
			Name:     "type",
			Required: true,
			Values:   []string{"good", "bad"},
		})

		collection.Fields.Add(&core.NumberField{
			Name:     "points",
			Required: false,
		})

		collection.Fields.Add(&core.RelationField{
			Name:          "userId",
			Required:      true,
			CollectionId:  "_pb_users_auth_",
			CascadeDelete: true,
		})

		collection.ListRule = ptrStr("userId = @request.auth.id")
		collection.ViewRule = ptrStr("userId = @request.auth.id")
		collection.CreateRule = ptrStr("@request.auth.id != \"\"")
		collection.UpdateRule = ptrStr("userId = @request.auth.id")
		collection.DeleteRule = ptrStr("userId = @request.auth.id")

		return app.Save(collection)
	}, func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("habits")
		if err != nil {
			return err
		}

		return app.Delete(collection)
	})
}

func ptrStr(s string) *string {
	return &s
}
