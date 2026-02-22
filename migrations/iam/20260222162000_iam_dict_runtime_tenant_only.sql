-- +goose Up
-- +goose StatementBegin
DROP POLICY IF EXISTS tenant_isolation ON iam.dicts;
CREATE POLICY tenant_isolation ON iam.dicts
USING (tenant_uuid = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_uuid = current_setting('app.current_tenant')::uuid);

DROP POLICY IF EXISTS tenant_isolation ON iam.dict_events;
CREATE POLICY tenant_isolation ON iam.dict_events
USING (tenant_uuid = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_uuid = current_setting('app.current_tenant')::uuid);

DROP POLICY IF EXISTS tenant_isolation ON iam.dict_value_segments;
CREATE POLICY tenant_isolation ON iam.dict_value_segments
USING (tenant_uuid = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_uuid = current_setting('app.current_tenant')::uuid);

DROP POLICY IF EXISTS tenant_isolation ON iam.dict_value_events;
CREATE POLICY tenant_isolation ON iam.dict_value_events
USING (tenant_uuid = current_setting('app.current_tenant')::uuid)
WITH CHECK (tenant_uuid = current_setting('app.current_tenant')::uuid);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP POLICY IF EXISTS tenant_isolation ON iam.dicts;
CREATE POLICY tenant_isolation ON iam.dicts
USING (
  tenant_uuid = current_setting('app.current_tenant')::uuid
  OR tenant_uuid = '00000000-0000-0000-0000-000000000000'::uuid
)
WITH CHECK (tenant_uuid = current_setting('app.current_tenant')::uuid);

DROP POLICY IF EXISTS tenant_isolation ON iam.dict_events;
CREATE POLICY tenant_isolation ON iam.dict_events
USING (
  tenant_uuid = current_setting('app.current_tenant')::uuid
  OR tenant_uuid = '00000000-0000-0000-0000-000000000000'::uuid
)
WITH CHECK (tenant_uuid = current_setting('app.current_tenant')::uuid);

DROP POLICY IF EXISTS tenant_isolation ON iam.dict_value_segments;
CREATE POLICY tenant_isolation ON iam.dict_value_segments
USING (
  tenant_uuid = current_setting('app.current_tenant')::uuid
  OR tenant_uuid = '00000000-0000-0000-0000-000000000000'::uuid
)
WITH CHECK (tenant_uuid = current_setting('app.current_tenant')::uuid);

DROP POLICY IF EXISTS tenant_isolation ON iam.dict_value_events;
CREATE POLICY tenant_isolation ON iam.dict_value_events
USING (
  tenant_uuid = current_setting('app.current_tenant')::uuid
  OR tenant_uuid = '00000000-0000-0000-0000-000000000000'::uuid
)
WITH CHECK (tenant_uuid = current_setting('app.current_tenant')::uuid);
-- +goose StatementEnd
