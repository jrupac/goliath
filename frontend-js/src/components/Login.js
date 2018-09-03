import React from 'react';
import {Form, Icon, Input, Button} from 'antd';

const FormItem = Form.Item;

class WrappedLogin extends React.Component {
  constructor(props) {
    super(props);
    this.state = {loginFailed: false};
  }

  render() {
    const {getFieldDecorator} = this.props.form;
    return (
      <div className='login-page'>
        <div className='login-page-logo'>Goliath</div>
        <div className='login-page-form-background'>
          <Form
            id="login-form"
            onSubmit={(e) => this.handleSubmit(e)}
            layout='vertical'
            className='login-page-form'>
            {this.showLoginFailedMessage()}
            <FormItem>
              {getFieldDecorator('username', {
                rules: [{required: true, message: 'Empty username!'}],
              })(<Input
                prefix={<Icon type="user" style={{fontSize: 13}}/>}
                placeholder="username"/>)}
            </FormItem>
            <FormItem>
              {getFieldDecorator('password', {
                rules: [{required: true, message: 'Empty password!'}],
              })(<Input
                prefix={<Icon type="lock" style={{fontSize: 13}}/>}
                type="password"
                placeholder="password"/>)}
            </FormItem>
            <Button
              type="primary"
              htmlType="submit"
              className="login-page-form-button">
              Log in
            </Button>
          </Form>
        </div>
      </div>
    )
  }

  showLoginFailedMessage = () => {
    if (this.state.loginFailed) {
      return <div className="login-failed">Invalid username or password!</div>
    } else {
      return null;
    }
  };

  handleSubmit(e) {
    e.preventDefault();
    this.props.form.validateFields((err, values) => {
      if (!err) {
        fetch('/auth', {
          method: 'POST',
          body: JSON.stringify(values),
          credentials: 'include'
        }).then((res) => {
          if (!res.ok) {
            this.setState({loginFailed: true});
            console.log(res);
          } else {
            this.setState({loginFailed: false});
            this.props.router.push({
              pathname: '/'
            });
          }
        }).catch((e) => {
          this.setState({loginFailed: true});
          console.log(e);
        });
      }
    });
  }
}

export default Form.create()(WrappedLogin);
