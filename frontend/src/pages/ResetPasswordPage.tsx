import React, { useState } from "react";
import { useNavigate, useSearchParams, Link } from "react-router-dom";
import { Lock, ArrowRight, ArrowLeft, ShieldCheck } from "lucide-react";
import { apiClient as api, APIError } from "../api/client";
import { TextField } from "../components/common/TextField";
import { Button } from "../components/common/Button";
import { Alert } from "../components/common/Alert";
import { EmptyState } from "../components/common/EmptyState";
import { AuthLayout } from "../components/auth/AuthLayout";
import styles from "../components/auth/AuthLayout.module.css";

const MIN_PASSWORD = 8;

type FieldErrors = { password?: string; confirmPassword?: string };

export const ResetPasswordPage: React.FC = () => {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [fieldErrors, setFieldErrors] = useState<FieldErrors>({});
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const [success, setSuccess] = useState(false);

  const token = searchParams.get("token");

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!token) return;

    const errors: FieldErrors = {};
    if (password.length < MIN_PASSWORD) {
      errors.password = `Use at least ${MIN_PASSWORD} characters.`;
    }
    if (password !== confirmPassword) {
      errors.confirmPassword = "Passwords do not match.";
    }
    setFieldErrors(errors);
    if (errors.password || errors.confirmPassword) return;

    setError("");
    setLoading(true);

    try {
      await api.post("/auth/reset-password", { token, password });
      setSuccess(true);
      setTimeout(() => {
        navigate("/login");
      }, 3000);
    } catch (err) {
      setError(
        err instanceof APIError && err.message
          ? err.message
          : "Failed to reset password",
      );
    } finally {
      setLoading(false);
    }
  };

  const backToLogin = (
    <Link to="/login" className={styles.mutedLink}>
      <ArrowLeft size={14} aria-hidden="true" /> Back to sign in
    </Link>
  );

  if (success) {
    return (
      <AuthLayout title="Password updated">
        <EmptyState
          compact
          tone="accent"
          icon={<ShieldCheck size={22} />}
          title="You're all set"
          description="Your password has been reset. Taking you to sign in…"
        />
        <Button size="lg" fullWidth onClick={() => navigate("/login")}>
          Go to Sign In
        </Button>
      </AuthLayout>
    );
  }

  return (
    <AuthLayout
      title="Set a new password"
      subtitle="Choose something you haven't used here before."
      footer={backToLogin}
    >
      {!token && (
        <Alert>
          This reset link is invalid or missing its token.{" "}
          <Link to="/forgot-password" className={styles.link}>
            Request a new link
          </Link>
          .
        </Alert>
      )}
      {error && <Alert>{error}</Alert>}

      <form onSubmit={handleSubmit} className={styles.form} noValidate>
        <TextField
          label="New password"
          type="password"
          name="new-password"
          autoComplete="new-password"
          placeholder={`At least ${MIN_PASSWORD} characters`}
          value={password}
          onChange={(e) => {
            setPassword(e.target.value);
            if (fieldErrors.password)
              setFieldErrors((f) => ({ ...f, password: undefined }));
          }}
          icon={<Lock size={16} />}
          error={fieldErrors.password}
          disabled={!token}
          revealable
          required
        />

        <TextField
          label="Confirm new password"
          type="password"
          name="confirm-password"
          autoComplete="new-password"
          placeholder="Type it again"
          value={confirmPassword}
          onChange={(e) => {
            setConfirmPassword(e.target.value);
            if (fieldErrors.confirmPassword)
              setFieldErrors((f) => ({ ...f, confirmPassword: undefined }));
          }}
          icon={<Lock size={16} />}
          error={fieldErrors.confirmPassword}
          disabled={!token}
          revealable
          required
        />

        <Button
          type="submit"
          size="lg"
          fullWidth
          isLoading={loading}
          disabled={!token}
          trailingIcon={<ArrowRight size={16} />}
        >
          Reset Password
        </Button>
      </form>
    </AuthLayout>
  );
};
