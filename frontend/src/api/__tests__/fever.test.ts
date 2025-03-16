import Fever from '../fever';
import { LoginInfo } from '../interface';
import Fever from '../fever';

describe('Fever', () => {
  it('initializes', () => {
    // Minimal test to check if the class can be initialized
    new Fever();
  });
});
describe('Fever', () => {
  let fever: Fever;
  let mockFetch: jest.Mock;

  beforeEach(() => {
    fever = new Fever();
    mockFetch = jest.fn();
    global.fetch = mockFetch;
  });

  it('HandleAuth returns true on successful login', async () => {
    const loginInfo: LoginInfo = { username: 'testuser', password: 'password' };
    mockFetch.mockResolvedValue({ ok: true });
    const result = await fever.HandleAuth(loginInfo);
    expect(result).toBe(true);
    expect(mockFetch).toHaveBeenCalledWith('/auth', {
      method: 'POST',
      body: JSON.stringify(loginInfo),
      credentials: 'include',
    });
  });

  it('HandleAuth returns false on failed login', async () => {
    const loginInfo: LoginInfo = { username: 'testuser', password: 'password' };
    mockFetch.mockResolvedValue({ ok: false });
    const result = await fever.HandleAuth(loginInfo);
    expect(result).toBe(false);
  });
});