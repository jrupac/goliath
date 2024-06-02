import React, {ChangeEvent, FormEvent, ReactNode} from 'react';
import {Navigate} from "react-router-dom";
import {
  Box,
  Button,
  CssBaseline,
  TextField,
  ThemeProvider
} from "@mui/material";
import {FetchAPI, FetchAPIFactory, LoginInfo} from "../api/interface";
import {GoliathPath, ThemeInfo} from "../utils/types";
import {populateThemeInfo} from "../utils/helpers";

export interface LoginProps {
}

export default class Login extends React.Component<LoginProps, any> {
  private fetchApi: FetchAPI;

  constructor(props: LoginProps) {
    super(props);
    this.state = {
      loginAttempted: false,
      loginSuccess: false,
      usernameMissing: false,
      passwordMissing: false,
      username: "",
      password: ""
    };
    this.fetchApi = FetchAPIFactory.Create();
  }

  render() {
    const themeInfo: ThemeInfo = populateThemeInfo(this.state.theme);

    if (this.state.loginSuccess) {
      return <Navigate to={GoliathPath.Default} replace={true}/>;
    }

    return (
      <ThemeProvider theme={themeInfo.theme}>
        <CssBaseline/>
        <Box className={`GoliathLoginPage ${themeInfo.themeClasses}`}>
          <Box className='GoliathLoginPageLogo'>Goliath</Box>
          <Box className='GoliathLoginPageSecondary'>
            <Box
              className="GoliathLoginPageForm">
              {this.showLoginFailedMessage()}
              <form
                onSubmit={(e: FormEvent) => this.handleSubmit(e)}
                className="login-page-form">
                <TextField
                  name="username"
                  label="Username"
                  error={this.state.usernameMissing}
                  value={this.state.username}
                  required
                  helperText={this.state.usernameMissing ? "Missing Username" : ""}
                  onChange={(e: ChangeEvent) => this.handleChange(e)}
                />
                <TextField
                  name="password"
                  label="Password"
                  type="password"
                  error={this.state.passwordMissing}
                  value={this.state.password}
                  required
                  helperText={this.state.passwordMissing ? "Missing Password" : ""}
                  onChange={(e: ChangeEvent) => this.handleChange(e)}
                />
                <Button
                  variant="contained"
                  type="submit"
                  onClick={(e: FormEvent) => this.handleSubmit(e)}
                  className="GoliathLoginFormButton">
                  Log in
                </Button>
              </form>
            </Box>
          </Box>
        </Box>
      </ThemeProvider>
    )
  }

  showLoginFailedMessage = (): ReactNode => {
    if (this.state.loginAttempted && !this.state.loginSuccess) {
      return <Box className="GoliathLoginFailedMessage">
        Invalid username or password.
      </Box>
    } else {
      return null;
    }
  };

  handleChange(e: any) {
    this.setState({
      [e.target.name]: e.target.value
    })
  }

  handleSubmit(e: React.FormEvent) {
    e.preventDefault();

    if (!this.validateForm()) {
      return;
    }

    const loginInfo: LoginInfo = {
      username: this.state.username,
      password: this.state.password
    }

    this.fetchApi.HandleAuth(loginInfo).then((ok: boolean) => {
      this.setState({
        loginAttempted: true,
        loginSuccess: ok
      });
    });
  }

  validateForm() {
    const usernameMissing = this.state.username === "";
    const passwordMissing = this.state.password === "";

    if (usernameMissing || passwordMissing) {
      this.setState({
        usernameMissing,
        passwordMissing
      });
      return false;
    }
    return true;
  }
}
