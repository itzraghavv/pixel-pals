-- name: CreateUser :exec
INSERT INTO users(id,username,password_hash,role) VALUES($1,$2,$3,$4);
-- name: UserByUsername :one
SELECT * FROM users WHERE username=$1;
-- name: UserByID :one
SELECT * FROM users WHERE id=$1;
-- name: UpdatePassword :exec
UPDATE users SET password_hash=$2 WHERE id=$1;
-- name: SetAvatar :exec
UPDATE users SET avatar_id=$2 WHERE id=$1;
-- name: BulkMetadata :many
SELECT u.id AS user_id, COALESCE(a.image_url,'')::text AS image_url FROM users u LEFT JOIN avatars a ON a.id=u.avatar_id WHERE u.id=ANY($1::text[]) ORDER BY u.id;
-- name: ListAvatars :many
SELECT * FROM avatars ORDER BY id;
-- name: GetAvatar :one
SELECT * FROM avatars WHERE id=$1;
-- name: CreateAvatar :exec
INSERT INTO avatars(id,name,image_url) VALUES($1,$2,$3);
-- name: ListElements :many
SELECT * FROM elements ORDER BY id;
-- name: GetElement :one
SELECT * FROM elements WHERE id=$1;
-- name: CreateElement :exec
INSERT INTO elements(id,width,height,static,image_url) VALUES($1,$2,$3,$4,$5);
-- name: UpdateElement :execrows
UPDATE elements SET image_url=$2 WHERE id=$1;
-- name: ListMaps :many
SELECT * FROM maps ORDER BY id;
-- name: GetMap :one
SELECT * FROM maps WHERE id=$1;
-- name: CreateMap :exec
INSERT INTO maps(id,name,width,height,thumbnail_url) VALUES($1,$2,$3,$4,$5);
-- name: CreateMapElement :exec
INSERT INTO map_elements(id,map_id,element_id,x,y) VALUES($1,$2,$3,$4,$5);
-- name: GetMapElements :many
SELECT * FROM map_elements WHERE map_id=$1 ORDER BY id;
-- name: CreateSpace :exec
INSERT INTO spaces(id,name,width,height,thumbnail_url,creator_id) VALUES($1,$2,$3,$4,$5,$6);
-- name: ListSpaces :many
SELECT * FROM spaces WHERE creator_id=$1 ORDER BY id;
-- name: GetSpace :one
SELECT * FROM spaces WHERE id=$1;
-- name: LockSpace :one
SELECT * FROM spaces WHERE id=$1 FOR UPDATE;
-- name: DeleteSpace :exec
DELETE FROM spaces WHERE id=$1;
-- name: GetSpaceElements :many
SELECT se.id,se.element_id,se.x,se.y,e.width,e.height,e.static,e.image_url FROM space_elements se JOIN elements e ON e.id=se.element_id WHERE se.space_id=$1 ORDER BY se.id;
-- name: GetPlacement :one
SELECT * FROM space_elements WHERE id=$1;
-- name: CreatePlacement :exec
INSERT INTO space_elements(id,space_id,element_id,x,y) VALUES($1,$2,$3,$4,$5);
-- name: DeletePlacement :exec
DELETE FROM space_elements WHERE id=$1;
-- name: AddMember :exec
INSERT INTO space_members(space_id,user_id,role) VALUES($1,$2,$3) ON CONFLICT DO NOTHING;
-- name: IsMember :one
SELECT EXISTS(SELECT 1 FROM space_members WHERE space_id=$1 AND user_id=$2);
-- name: CreateInvite :exec
INSERT INTO space_invites(token_hash,space_id,expires_at) VALUES($1,$2,$3);
-- name: GetInvite :one
SELECT * FROM space_invites WHERE token_hash=$1 AND expires_at>now();
-- name: CreateRefresh :exec
INSERT INTO refresh_tokens(id,user_id,token_hash,expires_at) VALUES($1,$2,$3,$4);
-- name: ConsumeRefresh :one
UPDATE refresh_tokens SET revoked_at=now() WHERE token_hash=$1 AND revoked_at IS NULL AND expires_at>now() RETURNING user_id;
-- name: RevokeRefresh :exec
UPDATE refresh_tokens SET revoked_at=now() WHERE token_hash=$1;
-- name: EnqueueSpaceEvent :exec
INSERT INTO space_events(space_id) VALUES($1);
-- name: PendingEvents :many
SELECT * FROM space_events ORDER BY id LIMIT 100;
-- name: DeleteEvent :exec
DELETE FROM space_events WHERE id=$1;
