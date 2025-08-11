import '@testing-library/jest-dom';

import { within, Screen } from '@testing-library/react';

export const expectTextInElement = (
  element: HTMLElement,
  text: string
): HTMLElement => {
  return within(element).getByText(text);
};

export const expectClass = (
  element: HTMLElement,
  closestSelector: string,
  className: string,
  hasClass: boolean = true
) => {
  const targetElement = element.closest(closestSelector);
  if (hasClass) {
    expect(targetElement).toHaveClass(className);
  } else {
    expect(targetElement).not.toHaveClass(className);
  }
};

export const expectPartialText = (screen: Screen, partialText: string) => {
  expect(screen.getByText(partialText, { exact: false })).toBeInTheDocument();
};
