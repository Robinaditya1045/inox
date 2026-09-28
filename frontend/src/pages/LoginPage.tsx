import React, { useState, useEffect } from "react";
import { Link, useNavigate, useLocation } from "react-router-dom";
import { useAuth } from "../hooks/useAuth";
import { TextField } from "../components/common/TextField";
import { Button } from "../components/common/Button";
import { Alert } from "../components/common/Alert";
import { AuthLayout } from "../components/auth/AuthLayout";
import { Mail, Lock } from "lucide-react";
import styles from "../components/auth/AuthLayout.module.css";

type FieldErrors = { email?: string; password?: string };

export const LoginPage: React.FC = () => {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [fieldErrors, setFieldErrors] = useState<FieldErrors>({});

  const { login, error, clearError, user, isLoading } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();

  const from =
    (location.state as { from?: { pathname: string } })?.from?.pathname || "/";

  useEffect(() => {
    if (user && !isLoading) {
      navigate(from, { replace: true });
    }
  }, [user, isLoading, navigate, from]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    clearError();

    const errors: FieldErrors = {};
    if (!email.trim()) errors.email = "Enter your email address.";
    if (!password) errors.password = "Enter your password.";
    setFieldErrors(errors);
    if (errors.email || errors.password) {
      document
        .getElementById(errors.email ? "login-email" : "login-password")
        ?.focus();
      return;
    }

    try {
      await login({ email, password });
      navigate(from, { replace: true });
    } catch {
      // Error is caught and set in AuthContext
    }
  };

  return (
    <AuthLayout
      title="Welcome back"
      subtitle="Sign in to pick up where your watch party left off."
      footer={
        <>
          New to Inox?{" "}
          <Link to="/signup" className={styles.link}>
            Create an account
          </Link>
        </>
      }
    >
      {error && <Alert>{error}</Alert>}

      <form onSubmit={handleSubmit} className={styles.form} noValidate>
        <TextField
          id="login-email"
          label="Email"
          type="email"
          name="email"
          autoComplete="email"
          inputMode="email"
          spellCheck={false}
          placeholder="name@example.com"
          value={email}
          onChange={(e) => {
            setEmail(e.target.value);
            if (fieldErrors.email)
              setFieldErrors((f) => ({ ...f, email: undefined }));
          }}
          icon={<Mail size={16} />}
          error={fieldErrors.email}
          required
        />

        <div className={styles.fieldRow}>
          <TextField
            id="login-password"
            label="Password"
            type="password"
            name="password"
            autoComplete="current-password"
            placeholder="Your password"
            value={password}
            onChange={(e) => {
              setPassword(e.target.value);
              if (fieldErrors.password)
                setFieldErrors((f) => ({ ...f, password: undefined }));
            }}
            icon={<Lock size={16} />}
            error={fieldErrors.password}
            revealable
            required
          />
          <Link
            to="/forgot-password"
            className={`${styles.mutedLink} ${styles.fieldAside}`}
          >
            Forgot password?
          </Link>
        </div>

        <Button
          type="submit"
          variant="primary"
          size="lg"
          fullWidth
          isLoading={isLoading}
        >
          Sign In
        </Button>
      </form>
    </AuthLayout>
  );
};
