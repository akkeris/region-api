-- Parameters
--  db_host: controller-api database host
--  db_name: controller-api database name
--  db_user: controller-api database username
--  db_pass: controller-api database password
CREATE FUNCTION pg_temp.v2_schema_migration(db_host text, db_name text, db_user text, db_pass text)
	RETURNS void AS 
$func$
BEGIN

	-- =================================================
	-- Foreign Data Wrapper to Controller API Database
	-- 
	-- NOTE:
	--   Temporary measure that will be removed
	--   when the v2 schema migration is complete.
	-- =================================================

	-- Make sure postgres_fdw extension exists
	create extension if not exists postgres_fdw;

	-- Create neccesary controller-api custom data types
	if not exists (select 1 from pg_type where typname = 'alpha_numeric') then
		create domain alpha_numeric as varchar(128) check (value ~ '^[A-z0-9\-]+$');
	end if;

	if not exists (select 1 from pg_type where typname = 'href') then
		create domain href as varchar(1024);
	end if;

	-- Create fdw server if not exists
	if not exists (
		select null from pg_foreign_server
				join pg_foreign_data_wrapper pgfdw on pgfdw.oid = srvfdw
				where fdwname = 'postgres_fdw' and srvname = 'controller_api_server'
	) then
		execute format('
			create server controller_api_server
				foreign data wrapper postgres_fdw options (host %L, dbname %L)
		', db_host, db_name);
	end if;

	-- Create fdw user mapping if not exists
	if not exists (
		select null from pg_user_mappings
			where srvname = 'controller_api_server' and usename = (select current_user)
	) then
		execute format('
			create user mapping for current_user
			server controller_api_server options (user %L, password %L);
		', db_user, db_pass);
	end if;

	-- Import foreign schema from controller api if not exists
	if not exists (
		select null from information_schema.schemata
			where schema_name = 'controller_api'
	) then
			create schema controller_api;
			import foreign schema public
					limit to (apps, spaces)
					from server controller_api_server
					into controller_api;
	end if;

	-- =================================================
	-- End FDW definition
	-- =================================================

	-- Create V2 Schema
	create schema if not exists v2;
	
	-- Create V2 Tables and Views
	if not exists (
			select null from information_schema.views
					where table_name = 'deployments'
					and table_schema = 'v2'
	) then
			create view v2.deployments as
					select controller_results.appid as appid,
							spacesapps.appname as name,
							spacesapps.space as space,
							spacesapps.plan as plan,
							spacesapps.instances as instances,
							spacesapps.healthcheck as healthcheck,
							spacesapps.port as port
					from spacesapps
					join (
							select apps.app as appid,
											apps.name as appname,
											spaces.name as spacename
							from controller_api.apps as apps
							join controller_api.spaces as spaces on apps.space = spaces.space
							where apps.deleted = false
					) as controller_results on spacesapps.appname = controller_results.appname 
							and spacesapps.space = controller_results.spacename;
	end if;

	-- Create functions, triggers to handle inserts, updates, deletes, etc. on the view

	create or replace function v2_deployments_insert_row() returns trigger as $v2_deployments_insert$
			begin
					-- Ignore the app ID and insert all other values into the spacesapps table
					insert into spacesapps(appname, space, plan, instances, healthcheck, port) 
							values(
									NEW.name,
									NEW.space,
									NEW.plan,
									NEW.instances,
									NEW.healthcheck,
									NEW.port
							);
					return NEW;
			end;
	$v2_deployments_insert$ language plpgsql;

	create or replace function v2_deployments_delete_row() returns trigger as $v2_deployments_delete$
			begin
					-- Delete row from the spacesapps table
					delete from spacesapps where spacesapps.appname = OLD.name and spacesapps.space = OLD.space;
					return OLD;
			end;
	$v2_deployments_delete$ language plpgsql;

	create or replace function v2_deployments_update_row() returns trigger as $v2_deployments_update$
			begin
					-- Update row in the spacesapps table, ignoring app ID
					update spacesapps set 
							appname = NEW.name,
							space = NEW.space,
							plan = NEW.plan,
							instances = NEW.instances,
							healthcheck = NEW.healthcheck,
							port = NEW.port
					where spacesapps.appname = OLD.name and spacesapps.space = OLD.space;
					return NEW;
			end;
	$v2_deployments_update$ language plpgsql;

	-- Override inserting rows on v2.deployments
	drop trigger if exists v2_deployments_insert on v2.deployments;
	create trigger v2_deployments_insert
			instead of insert on v2.deployments
			for each row
			execute procedure v2_deployments_insert_row();

	-- Override deleting rows on v2.deployments
	drop trigger if exists v2_deployments_delete on v2.deployments;
	create trigger v2_deployments_delete
			instead of delete on v2.deployments
			for each row
			execute procedure v2_deployments_delete_row();

	-- Override updating rows on v2.deployments
	drop trigger if exists v2_deployments_update on v2.deployments;
	create trigger v2_deployments_update
			instead of update on v2.deployments
			for each row
			execute procedure v2_deployments_update_row();

END
$func$ LANGUAGE plpgsql;