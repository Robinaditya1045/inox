import React, { useState, useEffect } from "react";
import { Link, useNavigate } from "react-router-dom";
import { useAuth } from "../hooks/useAuth";
import { TextField } from "../components/common/TextField";
import { Button } from "../components/common/Button";
import { Alert } from "../components/common/Alert";
import { AuthLayout } from "../components/auth/AuthLayout";
import { Mail, Lock, User as UserIcon } from "lucide-react";
import styles from "../components/auth/AuthLayout.module.css";

type Field = "username" | "email" | "password" | "confirmPassword";
type FieldErrors = Partial<Record<Field, string>>;

const MIN_PASSWORD = 8;

export const SignupPage: React.FC = () => {
  const [username, setUsername] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [fieldErrors, setFieldErrors] = useState<FieldErrors>({});

  const { signup, error, clearError, user, isLoading } = useAuth();
  const navigate = useNavigate();

  useEffect(() => {
    if (user && !isLoading) {
      navigate("/", { replace: true });
    }
  }, [user, isLoading, navigate]);

  const clearField = (field: Field) => {
    if (fieldErrors[field]) {
      setFieldErrors((f) => ({ ...f, [field]: undefined }));
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    clearError();

    const errors: FieldErrors = {};
    if (!username.trim()) errors.username = "Choose a username.";
    if (!email.trim()) errors.email = "Enter your email address.";
    if (!password) errors.password = "Choose a password.";
    else if (password.length < MIN_PASSWORD)
      errors.password = `Use at least ${MIN_PASSWORD} characters.`;
    if (password && password !== confirmPassword)
      errors.confirmPassword = "Passwords do not match.";

    setFieldErrors(errors);
    const firstInvalid = (
      ["username", "email", "password", "confirmPassword"] as Field[]
    ).find((f) => errors[f]);
    if (firstInvalid) {
      document.getElementById(`signup-${firstInvalid}`)?.focus();
      return;
    }

    try {
      await signup({ username, email, password });
      navigate("/", { replace: true });
    } catch {
      // Error is handled by AuthContext
    }
  };

  return (
    <AuthLayout
      title="Create your account"
      subtitle="Host watch parties, talk over voice and share your screen."
      footer={
        <>
          Already have an account?{" "}
          <Link to="/login" className={styles.link}>
            Sign in
          </Link>
        </>
      }
    >
      {error && <Alert>{error}</Alert>}

      <form onSubmit={handleSubmit} className={styles.form} noValidate>
        <TextField
          id="signup-username"
          label="Username"
          type="text"
          name="username"
          autoComplete="username"
          autoCapitalize="none"
          spellCheck={false}
          placeholder="nightowl_42"
          value={username}
          onChange={(e) => {
            setUsername(e.target.value);
            clearField("username");
          }}
          icon={<UserIcon size={16} />}
          error={fieldErrors.username}
          helperText="This is how friends will find you."
          required
        />

        <TextField
          id="signup-email"
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
            clearField("email");
          }}
          icon={<Mail size={16} />}
          error={fieldErrors.email}
          required
        />

        <TextField
          id="signup-password"
          label="Password"
          type="password"
          name="new-password"
          autoComplete="new-password"
          placeholder={`At least ${MIN_PASSWORD} characters`}
          value={password}
          onChange={(e) => {
            setPassword(e.target.value);
            clearField("password");
          }}
          icon={<Lock size={16} />}
          error={fieldErrors.password}
          revealable
          required
        />

        <TextField
          id="signup-confirmPassword"
          label="Confirm password"
          type="password"
          name="confirm-password"
          autoComplete="new-password"
          placeholder="Type it again"
          value={confirmPassword}
          onChange={(e) => {
            setConfirmPassword(e.target.value);
            clearField("confirmPassword");
          }}
          icon={<Lock size={16} />}
          error={fieldErrors.confirmPassword}
          revealable
          required
        />

        <Button
          type="submit"
          variant="primary"
          size="lg"
          fullWidth
          isLoading={isLoading}
        >
          Create Account
        </Button>
      </form>
    </AuthLayout>
  );
};
