import React, {ChangeEvent, FormEvent, ReactNode} from 'react';
import {RouteComponentProps, withRouter} from "react-router-dom";
import {
  Box,
  Button,
  createTheme,
  CssBaseline,
  TextField,
  ThemeProvider
} from "@mui/material";
import {FetchAPI, FetchAPIFactory, LoginInfo} from "../api/interface";
import {DefaultPath} from "../utils/helpers";

// WrappedLoginProps needs to extend RouteComponentProps to get "history".
export interface WrappedLoginProps extends RouteComponentProps {
}

class WrappedLogin extends React.Component<WrappedLoginProps, any> {
  private fetchApi: FetchAPI;

  constructor(props: WrappedLoginProps) {
    super(props);
    this.state = {
      loginFailed: false,
      usernameMissing: false,
      passwordMissing: false,
      username: "",
      password: ""
    };
    this.fetchApi = FetchAPIFactory.Create();
  }

  render() {
    const theme = createTheme();

    return (
      <ThemeProvider theme={theme}>
        <CssBaseline/>
        <Box className='GoliathLoginPage'>
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
    if (this.state.loginFailed) {
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

    this.fetchApi.HandleLogin(loginInfo).then((ok: boolean) => {
      if (ok) {
        this.setState({loginFailed: false});
        this.props.history.push({
          pathname: DefaultPath
        });
      } else {
        this.setState({loginFailed: true});
      }
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

export default withRouter(WrappedLogin);
