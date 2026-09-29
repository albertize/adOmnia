import { describe, expect, it } from 'vitest'
import type { DbConnection } from '@/components/database/dbShared'
import { databaseOperationNeedsConfirmation, extensionBrokerPublishRequest } from './ExtensionContributionHost'

const connection = (driver: DbConnection['driver']) => ({ driver } as DbConnection)

describe('extension broker publish requests', () => {
  it('maps saved profiles to canonical sidecar publish contracts', () => {
    const kafka = extensionBrokerPublishRequest({ id: 'k', name: 'Kafka', protocol: 'kafka', updatedAt: '', config: { brokers: 'one:9092, two:9092', saslEnabled: false } }, 'events', 'hello', { key: 'id-1' })
    expect(kafka.path).toBe('/kafka/produce')
    expect(kafka.body).toMatchObject({ key: 'id-1', value: 'hello', config: { brokers: ['one:9092', 'two:9092'], topic: 'events' } })

    const mqtt = extensionBrokerPublishRequest({ id: 'm', name: 'MQTT', protocol: 'mqtt', updatedAt: '', config: { broker: 'tcp://localhost:1883' } }, 'devices/1', 'online', { qos: 2, retained: true })
    expect(mqtt).toEqual({ path: '/broker/mqtt/publish', body: { config: { broker: 'tcp://localhost:1883' }, topic: 'devices/1', payload: 'online', qos: 2, retained: true } })
  })
})

describe('extension database execution confirmation', () => {
  it('allows common read-only SQL without an extra confirmation', () => {
    expect(databaseOperationNeedsConfirmation(connection('sqlite'), 'SELECT * FROM users')).toBe(false)
    expect(databaseOperationNeedsConfirmation(connection('postgres'), 'EXPLAIN SELECT 1')).toBe(false)
  })

  it('requires confirmation for every SQL mutation, including scoped writes', () => {
    expect(databaseOperationNeedsConfirmation(connection('sqlite'), 'INSERT INTO users(name) VALUES (\'Ada\')')).toBe(true)
    expect(databaseOperationNeedsConfirmation(connection('postgres'), 'UPDATE users SET active = true WHERE id = 1')).toBe(true)
    expect(databaseOperationNeedsConfirmation(connection('mysql'), 'DROP TABLE users')).toBe(true)
  })

  it('allows MongoDB reads and confirms writes or unknown operations', () => {
    expect(databaseOperationNeedsConfirmation(connection('mongodb'), '{"operation":"find","collection":"users"}')).toBe(false)
    expect(databaseOperationNeedsConfirmation(connection('mongodb'), '{"operation":"insertOne","collection":"users"}')).toBe(true)
    expect(databaseOperationNeedsConfirmation(connection('mongodb'), '{"collection":"users"}')).toBe(true)
  })
})
