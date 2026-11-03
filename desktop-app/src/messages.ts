import type { TFunction } from "i18next";

export interface MessageDescriptor {
  key: string;
  values?: Record<string, string | number>;
}
export type UiMessage = string | MessageDescriptor;

export class AppMessageError extends Error {
  constructor(public readonly descriptor: MessageDescriptor) {
    super(descriptor.key);
    this.name = "AppMessageError";
  }
}

export function toUiMessage(error: unknown): UiMessage {
  return error instanceof AppMessageError ? error.descriptor : String(error);
}

export function messageText(message: UiMessage, t: TFunction): string {
  return typeof message === "string" ? message : t(message.key, message.values ?? {});
}
