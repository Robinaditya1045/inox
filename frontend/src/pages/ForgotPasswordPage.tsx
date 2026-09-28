import React, { useState } from "react";
import { useNavigate, Link } from "react-router-dom";
import { Mail, ArrowRight, ArrowLeft, MailCheck } from "lucide-react";
import { apiClient as api, APIError } from "../api/client";
import { TextField } from "../components/common/TextField";
import { Button } from "../components/common/Button";
import { EmptyState } from "../components/common/EmptyState";
import { AuthLayout } from "../components/auth/AuthLayout";
import styles from "../components/auth/AuthLayout.module.css";

export const ForgotPasswordPage: React.FC = () => {
  const navigate = useNavigate();
  const [email, setEmail] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const [success, setSuccess] = useState(false);
  const [token, setToken] = useState("");

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    if (!email.trim()) {
      setError("Enter the email address on your account.");
      return;
    }
    setLoading(true);

    try {
      const response = await api.post<{ token?: string }>(
        "/auth/forgot-password",
        { email },
      );
      setSuccess(true);
      if (response.token) {
        setToken(response.token);
      }
    } catch (err) {
      setError(
        err instanceof APIError && err.message
          ? err.message
          : "Failed to process request",
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
      <AuthLayout title="Check your inbox" footer={backToLogin}>
        <EmptyState
          compact
          tone="accent"
          icon={<MailCheck size={22} />}
          title="Reset link on its way"
          description="If an account exists for that address, we've sent a link to reset your password."
        />

        {token && (
          <div className={styles.token}>
            <span className={styles.tokenLabel}>Demo token</span>
            {token}
          </div>
        )}

        <Button
          size="lg"
          fullWidth
          trailingIcon={<ArrowRight size={16} />}
          onClick={() =>
            navigate(token ? `/reset-password?token=${token}` : "/login")
          }
        >
          {token ? "Continue to Reset" : "Return to Sign In"}
        </Button>
      </AuthLayout>
    );
  }

  return (
    <AuthLayout
      title="Forgot your password?"
      subtitle="Enter the email you signed up with and we'll send you a reset link."
      footer={backToLogin}
    >
      <form onSubmit={handleSubmit} className={styles.form} noValidate>
        <TextField
          label="Email"
          type="email"
          name="email"
          autoComplete="email"
          inputMode="email"
          spellCheck={false}
          value={email}
          onChange={(e) => {
            setEmail(e.target.value);
            if (error) setError("");
          }}
          placeholder="name@example.com"
          icon={<Mail size={16} />}
          error={error || undefined}
          required
        />

        <Button
          type="submit"
          size="lg"
          fullWidth
          isLoading={loading}
          trailingIcon={<ArrowRight size={16} />}
        >
          Send Reset Link
        </Button>
      </form>
    </AuthLayout>
  );
};
