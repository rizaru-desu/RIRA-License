ALTER TABLE verification_tokens
    DROP CONSTRAINT IF EXISTS verification_tokens_type_check,
    ADD CONSTRAINT verification_tokens_type_check
        CHECK (type IN ('email_verification', 'password_reset', 'email_change', 'account_deletion'));
