# frozen_string_literal: true

require 'copilot/protos/cloud_controller_future_pb'
require 'copilot/protos/cloud_controller_future_services_pb'

module Cloudfoundry
  module Copilot
    class Client

      attr_reader :host, :port

      def initialize(host:, port:, client_ca:, client_key:, client_chain:, timeout: 5)
        @host = host
        @port = port
        @url = "#{host}:#{port}"
        @timeout = timeout
        @client_ca = client_ca
        @client_key = client_key
        @client_chain = client_chain
      end

      def upsert_route(guid:, host:, path:)
        request = Protos::UpsertRouteRequest.new(guid: guid, host: host, path: path)
        service.upsert_route(request)

      rescue GRPC::BadStatus => e
        raise puts "error code: '#{e.code}' occurred due to '#{e.details}' with metadata '#{e.metadata}'"
      end

      def delete_route(guid:)
        request = Protos::DeleteRouteRequest.new(guid: guid)
        service.delete_route(request)

      rescue GRPC::BadStatus => e
        raise puts "error code: '#{e.code}' occurred due to '#{e.details}' with metadata '#{e.metadata}'"
      end

      def map_route(guid:, app_guid:, route_guid:)
        process = Protos::MapRouteRequest::Process.new(guid: guid, app_guid: app_guid)
        request = Protos::MapRouteRequest.new(process: process, route_guid: route_guid)
        service.map_route(request)

      rescue GRPC::BadStatus => e
        raise puts "error code: '#{e.code}' occurred due to '#{e.details}' with metadata '#{e.metadata}'"
      end

      def unmap_route(process_guid:, route_guid:)
        request = Protos::UnmapRouteRequest.new(process_guid: process_guid, route_guid: route_guid)
        service.map_route(request)

      rescue GRPC::BadStatus => e
        raise puts "error code: '#{e.code}' occurred due to '#{e.details}' with metadata '#{e.metadata}'"
      end

      def bulk_sync(routes:, route_mappings:)
        routes.map! { |route| Protos::UpsertRouteRequest.new(route: route) }
        route_mappings.map! { |mapping| Protos::MapRouteRequest.new(route_mapping: mapping) }

        request = Protos::BulkSyncRequest.new(routes: routes, route_mappings: route_mappings)
        service.bulk_sync(request)

      rescue GRPC::BadStatus => e
        raise puts "error code: '#{e.code}' occurred due to '#{e.details}' with metadata '#{e.metadata}'"
      end

      private

      def tls_credentials
        @tls_credentials ||= GRPC::Core::ChannelCredentials.new(@client_ca, @client_key, @client_chain)
      end

      def service
        @service ||= Protos::CloudControllerCopilotFuture::Stub.new(@url, tls_credentials, timeout: @timeout)
      end
    end
  end
end
