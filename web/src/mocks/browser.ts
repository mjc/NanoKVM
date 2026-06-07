import { setupWorker } from 'msw/browser'
import { http, HttpResponse } from 'msw'

export const handlers = [
  http.post('/api/auth/login', () => {
    return HttpResponse.json({
        code: 0,
        data: {
            token: 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.mock.signature',
        },
    })
  }),
]
export const worker = setupWorker(...handlers)
