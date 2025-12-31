package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		collection := core.NewBaseCollection("instruments")

		collection.Fields.Add(&core.RelationField{
			Name:          "userId",
			Required:      true,
			CollectionId:  "_pb_users_auth_",
			CascadeDelete: true,
			MaxSelect:     1,
		})

		collection.Fields.Add(&core.TextField{
			Name:     "name",
			Required: true,
			Min:      1,
			Max:      100,
		})

		collection.Fields.Add(&core.TextField{
			Name:     "description",
			Required: false,
			Max:      1000,
		})

		collection.Fields.Add(&core.JSONField{
			Name:     "instrumentData",
			Required: true,
			MaxSize:  2000000,
		})

		collection.Fields.Add(&core.BoolField{
			Name:     "isPublic",
			Required: false,
		})

		collection.Fields.Add(&core.JSONField{
			Name:     "tags",
			Required: false,
			MaxSize:  10000,
		})

		collection.Fields.Add(&core.AutodateField{
			Name:     "created",
			OnCreate: true,
			OnUpdate: false,
		})

		collection.Fields.Add(&core.AutodateField{
			Name:     "updated",
			OnCreate: true,
			OnUpdate: true,
		})

		collection.ListRule = ptrStr("@request.auth.id != '' && (userId = @request.auth.id || isPublic = true)")
		collection.ViewRule = ptrStr("@request.auth.id != '' && (userId = @request.auth.id || isPublic = true)")
		collection.CreateRule = ptrStr("@request.auth.id != ''")
		collection.UpdateRule = ptrStr("@request.auth.id != '' && userId = @request.auth.id")
		collection.DeleteRule = ptrStr("@request.auth.id != '' && userId = @request.auth.id")

		return app.Save(collection)
	}, func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("instruments")
		if err != nil {
			return err
		}

		return app.Delete(collection)
	})
}
