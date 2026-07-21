                                 Table "public.users"
     Column      |            Type             | Collation | Nullable |    Default    
-----------------+-----------------------------+-----------+----------+---------------
 id              | uuid                        |           | not null | 
 created_at      | timestamp without time zone |           | not null | 
 updated_at      | timestamp without time zone |           | not null | 
 email           | text                        |           | not null | 
 password_hash   | text                        |           | not null | 
 hashed_password | text                        |           | not null | 'unset'::text
Indexes:
    "users_pkey" PRIMARY KEY, btree (id)
    "users_email_key" UNIQUE CONSTRAINT, btree (email)
Referenced by:
    TABLE "chirps" CONSTRAINT "chirps_user_id_fkey" FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE

