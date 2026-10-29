import { FormEvent, ReactNode, useState } from 'react';
import { Navigate } from 'react-router';
import { Button, CssBaseline, TextField, ThemeProvider } from '@mui/material';
import { FetchAPI, FetchAPIFactory, LoginInfo } from '../api/interface';
import { GoliathPath, ThemeInfo } from '../utils/types';
import { populateThemeInfo } from '../utils/helpers';

const Login = () => {
  const [loginAttempted, setLoginAttempted] = useState<boolean>(false);
  const [loginSuccess, setLoginSuccess] = useState<boolean>(false);
  const [usernameMissing, setUsernameMissing] = useState<boolean>(false);
  const [passwordMissing, setPasswordMissing] = useState<boolean>(false);
  const [username, setUsername] = useState<string>('');
  const [password, setPassword] = useState<string>('');
  const [theme] = useState<any>(null);
  const fetchApi: FetchAPI = FetchAPIFactory.Create();

  const showLoginFailedMessage = (): ReactNode => {
    if (loginAttempted && !loginSuccess) {
      return (
        <div className="GoliathLoginFailedMessage">
          Invalid username or password.
        </div>
      );
    } else {
      return null;
    }
  };

  const handleChange = (e: any) => {
    const { name, value } = e.target;
    if (name === 'username') {
      setUsername(value);
    } else if (name === 'password') {
      setPassword(value);
    }
  };

  const validateForm = () => {
    const usernameMissingValue = username === '';
    const passwordMissingValue = password === '';

    if (usernameMissingValue || passwordMissingValue) {
      setUsernameMissing(usernameMissingValue);
      setPasswordMissing(passwordMissingValue);
      return false;
    }
    return true;
  };

  const handleSubmit = (e: FormEvent) => {
    e.preventDefault();

    if (!validateForm()) {
      return;
    }

    const loginInfo: LoginInfo = {
      username: username,
      password: password,
    };

    fetchApi.HandleAuth(loginInfo).then((ok: boolean) => {
      setLoginAttempted(true);
      setLoginSuccess(ok);
    });
  };
  const themeInfo: ThemeInfo = populateThemeInfo(theme);

  if (loginSuccess) {
    return <Navigate to={GoliathPath.Default} replace={true} />;
  }

  return (
    <ThemeProvider theme={themeInfo.theme}>
      <CssBaseline />
      <div className={`GoliathLoginPage ${themeInfo.themeClasses}`}>
        <div className="GoliathLoginPageLogo">Goliath</div>
        <div className="GoliathLoginPageSecondary">
          <div className="GoliathLoginPageForm">
            {showLoginFailedMessage()}
            <form onSubmit={handleSubmit} className="login-page-form">
              <TextField
                name="username"
                label="Username"
                error={usernameMissing}
                value={username}
                required
                helperText={usernameMissing ? 'Missing Username' : ''}
                onChange={handleChange}
              />
              <TextField
                name="password"
                label="Password"
                type="password"
                error={passwordMissing}
                value={password}
                required
                helperText={passwordMissing ? 'Missing Password' : ''}
                onChange={handleChange}
              />
              <Button
                variant="contained"
                type="submit"
                onClick={handleSubmit}
                className="GoliathLoginFormButton"
              >
                Log in
              </Button>
            </form>
          </div>
        </div>
      </div>
    </ThemeProvider>
  );
};

export default Login;
