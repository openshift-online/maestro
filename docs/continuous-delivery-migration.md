## CD Migration

Migrations can be problematic when trying to move towards continuous deployment. At first glance it looks impossible to CD a breaking migration. Essentially the solution is always to break the migration into multiple smaller and safer steps. Note that migrations can and should be tested -- like any other code change. In AMS we have a lot of experience testing complicated migrations:

https://gitlab.cee.redhat.com/service/uhc-account-manager/blob/master/pkg/db/README.md#migration-tests

https://gitlab.cee.redhat.com/service/uhc-account-manager/blob/master/test/integration/migrations_test.go

### Example

In this example from AMS we drop the `subscription_managed` field from `resource_quota` table and AMS API. This change is not backwards compatible as any existing running image of AMS will error if we simply drop the column from the database table. The solution is to break this migration down into multiple steps.

In AMS we deploy our services on pods running RollingUpdate. When deploying a code change each pod will roll and try to run migrations as part of the init container. We ensure migrations run only once by relying on [postgres' advisory lock](https://gitlab.cee.redhat.com/service/uhc-account-manager/blob/master/pkg/db/migrations.go#L193).

In order to deploy this change in a CD way we must first merge the change removing support for the field in the API. That code change needs to propagate through to all production service pods and cronjob pods:

https://issues.redhat.com/browse/SDB-849

https://gitlab.cee.redhat.com/service/uhc-account-manager/-/merge_requests/1213

Note that the merge request was merged Jan 22 2020. After the code change is fully deployed we drop the field from the database it is no longer used by any AMS code. Note that the second merge request was merged nearly 2 weeks later on Feb 3 2020:

https://issues.redhat.com/browse/SDB-858

https://gitlab.cee.redhat.com/service/uhc-account-manager/-/merge_requests/1214
